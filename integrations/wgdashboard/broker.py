"""CloudMateHub identity boundary for WGDashboard. No upstream secrets reach ordinary users."""
import os
import re
import secrets
import sqlite3
import threading
import time
from functools import wraps
from pathlib import Path
from urllib.parse import quote, urlsplit

import requests
from flask import Flask, Response, abort, g, jsonify, request

PREFIX = '/api/vpn'
DASH = PREFIX + '/dashboard'
COOKIE = 'cmh_vpn_session'


def create_app(config=None):
    app = Flask(__name__)
    app.config.update(
        CLOUDMATE_URL=os.getenv('CLOUDMATE_URL', 'http://server:8080'),
        WGDASHBOARD_URL=os.getenv('WGDASHBOARD_URL', 'http://wgdashboard:10086'),
        KEY_FILE=os.getenv('WG_KEY_FILE', '/run/secrets/wgdashboard_key'),
        DB_FILE=os.getenv('VPN_DB_FILE', '/data/assignments.db'),
        SESSION_TTL=300,
        MAX_CONTENT_LENGTH=64 * 1024 * 1024,
        COOKIE_SECURE=os.getenv('VPN_COOKIE_SECURE', 'false') == 'true',
    )
    if config:
        app.config.update(config)
    from werkzeug.middleware.proxy_fix import ProxyFix
    app.wsgi_app = ProxyFix(app.wsgi_app, x_for=0, x_proto=1, x_host=0)
    sessions = {}
    lock = threading.Lock()
    Path(app.config['DB_FILE']).parent.mkdir(parents=True, exist_ok=True)

    def db():
        conn = sqlite3.connect(app.config['DB_FILE'])
        conn.row_factory = sqlite3.Row
        return conn

    with db() as conn:
        conn.execute('''CREATE TABLE IF NOT EXISTS assignments
          (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, configuration TEXT NOT NULL,
           peer_id TEXT NOT NULL, label TEXT NOT NULL, created_by TEXT NOT NULL,
           UNIQUE(configuration, peer_id))''')

    os.chmod(app.config['DB_FILE'], 0o600)

    def same_origin():
        # Cookie-authenticated upstream GETs can mutate, so protect every proxy request.
        origin = request.headers.get('Origin')
        referer = request.headers.get('Referer')
        expected = request.host_url.rstrip('/')
        if origin:
            return origin == expected
        if referer:
            parsed = urlsplit(referer)
            return f'{parsed.scheme}://{parsed.netloc}' == expected
        return request.headers.get('Sec-Fetch-Site') == 'same-origin'

    def identity(token):
        if not token or len(token) > 16384:
            abort(401)
        try:
            response = requests.get(app.config['CLOUDMATE_URL'] + '/api/auth/user',
                                    headers={'Authorization': 'Bearer ' + token},
                                    timeout=10, allow_redirects=False)
        except requests.RequestException:
            abort(503, 'CloudMateHub identity service unavailable')
        if response.status_code != 200:
            abort(401)
        try:
            user = response.json()
        except ValueError:
            abort(502)
        if not isinstance(user, dict) or not isinstance(user.get('id'), str) or not user['id']:
            abort(401)
        return user

    def authenticated(admin=False, cookie=False):
        def decorate(fn):
            @wraps(fn)
            def wrapped(*args, **kwargs):
                authorization = request.headers.get('Authorization', '')
                token = authorization[7:] if authorization.startswith('Bearer ') else None
                if cookie and not token:
                    if not same_origin():
                        abort(403)
                    sid = request.cookies.get(COOKIE, '')
                    with lock:
                        now = time.monotonic()
                        for key in list(sessions):
                            if sessions[key][1] <= now:
                                del sessions[key]
                        entry = sessions.get(sid)
                    token = entry[0] if entry else None
                g.user = identity(token)
                g.token = token
                if admin and g.user.get('admin') is not True:
                    abort(403)
                return fn(*args, **kwargs)
            return wrapped
        return decorate

    def wg(method, path, content_type=None, **kwargs):
        key = Path(app.config['KEY_FILE']).read_text().strip()
        try:
            response = requests.request(method, app.config['WGDASHBOARD_URL'] + DASH + path,
                                        headers={'wg-dashboard-apikey': key, **({'Content-Type': content_type} if content_type else {})},
                                        timeout=(10, 60), allow_redirects=False, **kwargs)
        except requests.RequestException:
            abort(503, 'WGDashboard is not available')
        return response

    def wg_json(method, path, **kwargs):
        response = wg(method, path, **kwargs)
        if response.status_code != 200:
            abort(502, 'WGDashboard rejected the request')
        try:
            payload = response.json()
        except ValueError:
            abort(502)
        if payload.get('status') is not True:
            abort(409, 'WGDashboard could not resolve the requested configuration or peer')
        return payload.get('data')

    @app.after_request
    def private_response(response):
        response.headers['Cache-Control'] = 'no-store'
        response.headers['X-Content-Type-Options'] = 'nosniff'
        # The embedded app uses same-origin fetches without an Origin header.
        # Preserve their referrer for browsers without Fetch Metadata support;
        # cross-origin requests still receive no referrer.
        response.headers['Referrer-Policy'] = 'same-origin'
        response.headers['X-Frame-Options'] = 'SAMEORIGIN'
        return response

    @app.errorhandler(Exception)
    def failure(error):
        from werkzeug.exceptions import HTTPException
        if isinstance(error, HTTPException):
            return jsonify(error=error.description), error.code
        # Do not print exceptions containing credentials, upstream URLs, or config bodies.
        return jsonify(error='VPN service failed'), 500

    @app.get('/health')
    def health():
        return jsonify(ready=True)

    @app.get(PREFIX + '/status')
    @authenticated()
    def status():
        ready = False
        try:
            ready = wg('GET', '/api/handshake').status_code == 200
        except Exception:
            pass
        return jsonify(connected=ready, admin=g.user.get('admin') is True,
                       deployment='local-preparation', version='4.3.4')

    @app.post(PREFIX + '/session')
    @authenticated(admin=True)
    def open_session():
        sid = secrets.token_urlsafe(32)
        with lock:
            now = time.monotonic()
            for key in list(sessions):
                if sessions[key][1] <= now:
                    del sessions[key]
            # One live proxy session per CloudMateHub token; no unbounded renewal accumulation.
            for key, entry in list(sessions.items()):
                if secrets.compare_digest(entry[0], g.token):
                    del sessions[key]
            sessions[sid] = (g.token, now + app.config['SESSION_TTL'])
        response = jsonify(url=DASH + '/', expires_in=app.config['SESSION_TTL'])
        response.set_cookie(COOKIE, sid, httponly=True, secure=app.config['COOKIE_SECURE'] or request.is_secure,
                            samesite='Strict', path=PREFIX, max_age=app.config['SESSION_TTL'])
        return response

    @app.delete(PREFIX + '/session')
    def close_session():
        if not same_origin():
            abort(403)
        with lock:
            sessions.pop(request.cookies.get(COOKIE, ''), None)
        response = jsonify(closed=True)
        response.delete_cookie(COOKIE, path=PREFIX, samesite='Strict')
        return response

    @app.route(DASH + '/', defaults={'path': ''}, methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'])
    @app.route(DASH + '/<path:path>', methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'])
    @authenticated(admin=True, cookie=True)
    def dashboard(path):
        if path == 'embedded.css':
            return Response(Path(__file__).with_name('embedded.css').read_text(), content_type='text/css')
        if path == 'api/requireAuthentication':
            return jsonify(status=True, data=False)
        if path == 'api/validateAuthentication':
            return jsonify(status=True, data=None)
        if path == 'api/signout':
            return close_session()
        upstream = wg(request.method, '/' + path, params=request.args,
                      data=request.get_data(), content_type=request.content_type)
        # The upstream owns mutation success; reconcile peer ownership only after it commits.
        if request.method == 'POST' and upstream.status_code == 200:
            try:
                succeeded = upstream.json().get('status') is True
            except (ValueError, AttributeError):
                succeeded = False
            body = request.get_json(silent=True) or {}
            if succeeded:
                with db() as conn:
                    if path == 'api/renameWireguardConfiguration':
                        old, new = body.get('ConfigurationName'), body.get('NewConfigurationName')
                        if isinstance(old, str) and isinstance(new, str):
                            conn.execute('UPDATE assignments SET configuration=? WHERE configuration=?', (new, old))
                    elif path == 'api/deleteWireguardConfiguration':
                        name = body.get('ConfigurationName')
                        if isinstance(name, str):
                            conn.execute('DELETE FROM assignments WHERE configuration=?', (name,))
                    elif path.startswith('api/deletePeers/'):
                        name = path[len('api/deletePeers/'):]
                        for peer in body.get('peers', []):
                            if isinstance(peer, str):
                                conn.execute('DELETE FROM assignments WHERE configuration=? AND peer_id=?', (name, peer))
        headers = {}
        for name in ['Content-Type', 'Content-Disposition', 'Content-Security-Policy']:
            if name in upstream.headers:
                headers[name] = upstream.headers[name]
        if 300 <= upstream.status_code < 400:
            location = upstream.headers.get('Location', '')
            if not location.startswith(DASH + '/'):
                abort(502, 'Unexpected WGDashboard redirect')
            headers['Location'] = location
        content = upstream.content
        if path == '' and upstream.status_code == 200 and 'text/html' in upstream.headers.get('Content-Type', ''):
            content = content.replace(b'</head>', b'<link rel="stylesheet" href="./embedded.css"></head>')
        return Response(content, status=upstream.status_code, headers=headers)

    @app.get(PREFIX + '/assignments')
    @authenticated()
    def assignments():
        with db() as conn:
            if request.args.get('all') == 'true':
                if g.user.get('admin') is not True:
                    abort(403)
                rows = conn.execute('SELECT * FROM assignments ORDER BY label').fetchall()
            else:
                rows = conn.execute('SELECT * FROM assignments WHERE user_id=? ORDER BY label', (g.user['id'],)).fetchall()
        return jsonify([dict(row) for row in rows])

    @app.get(PREFIX + '/users')
    @authenticated(admin=True)
    def users():
        try:
            response = requests.get(app.config['CLOUDMATE_URL'] + '/internal/vpn/users',
                                    params={'search': request.args.get('search', ''), 'page': request.args.get('page', '1')},
                                    headers={'Authorization': 'Bearer ' + g.token}, timeout=10, allow_redirects=False)
        except requests.RequestException:
            abort(503)
        if response.status_code != 200:
            abort(503, 'CloudMateHub user directory is not available')
        data = response.json()
        if not isinstance(data, list):
            abort(502)
        return jsonify(data)

    @app.get(PREFIX + '/catalog')
    @authenticated(admin=True)
    def catalog():
        configurations = wg_json('GET', '/api/getWireguardConfigurations')
        result = []
        for configuration in configurations:
            name = configuration.get('Name')
            if not isinstance(name, str):
                continue
            details = wg_json('GET', '/api/getWireguardConfigurationInfo', params={'configurationName': name})
            peers = details.get('configurationPeers', []) + details.get('configurationRestrictedPeers', [])
            result.append({'name': name, 'peers': [
                {key: peer.get(key) for key in ['id', 'name', 'allowed_ip', 'status']} for peer in peers]})
        return jsonify(result)

    @app.post(PREFIX + '/assignments')
    @authenticated(admin=True)
    def assign():
        data = request.get_json(silent=True) or {}
        values = [data.get(k) for k in ['user_id', 'configuration', 'peer_id', 'label']]
        if any(not isinstance(v, str) or not v.strip() or len(v) > 255 for v in values):
            abort(400, 'User, configuration, peer and label are required')
        user_id, configuration, peer_id, label = values
        if not re.fullmatch(r'[A-Za-z0-9_-]+', configuration) or not re.fullmatch(r'[A-Za-z0-9_-]+', user_id):
            abort(400)
        try:
            target = requests.get(app.config['CLOUDMATE_URL'] + '/internal/vpn/users/' + quote(user_id, safe=''),
                                  headers={'Authorization': 'Bearer ' + g.token}, timeout=10, allow_redirects=False)
        except requests.RequestException:
            abort(503)
        if target.status_code != 200:
            abort(400, 'Select an existing CloudMateHub user')
        wg_json('GET', '/api/downloadPeer/' + quote(configuration, safe=''), params={'id': peer_id})
        try:
            with db() as conn:
                conn.execute('INSERT INTO assignments VALUES (?,?,?,?,?,?)',
                             (secrets.token_hex(16), user_id, configuration, peer_id, label, g.user['id']))
        except sqlite3.IntegrityError:
            abort(409, 'This peer is already assigned. Unassign it before changing the owner.')
        return jsonify(created=True), 201

    @app.delete(PREFIX + '/assignments/<assignment_id>')
    @authenticated(admin=True)
    def unassign(assignment_id):
        with db() as conn:
            conn.execute('DELETE FROM assignments WHERE id=?', (assignment_id,))
        return jsonify(removed=True, note='Download access removed; revoke the peer in WGDashboard to stop VPN access.')

    @app.get(PREFIX + '/assignments/<assignment_id>/configuration')
    @authenticated()
    def configuration(assignment_id):
        with db() as conn:
            row = conn.execute('SELECT * FROM assignments WHERE id=? AND user_id=?',
                               (assignment_id, g.user['id'])).fetchone()
        if row is None:
            abort(404)
        payload = wg_json('GET', '/api/downloadPeer/' + quote(row['configuration'], safe=''),
                          params={'id': row['peer_id']})
        if not isinstance(payload, dict) or not isinstance(payload.get('file'), str):
            abort(502)
        return jsonify(file=payload['file'], fileName=re.sub(r'[^A-Za-z0-9_.-]', '_', row['label']) + '.conf')

    return app


if __name__ != '__main__' and os.getenv('VPN_DB_FILE'):
    app = create_app()
