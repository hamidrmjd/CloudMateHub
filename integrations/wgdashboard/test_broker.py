import importlib.util
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import requests

spec = importlib.util.spec_from_file_location('broker', Path(__file__).with_name('broker.py'))
broker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(broker)
USER = 'a' * 24
OTHER = 'b' * 24
ADMIN = 'c' * 24

class Reply:
    def __init__(self, data=None, status=200, headers=None, content=b'upstream'):
        self.data, self.status_code, self.headers, self.content = data, status, headers or {}, content
    def json(self):
        return self.data

class BrokerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='cloudmatehub-vpn-test-')
        self.key = Path(self.tmp.name) / 'key'
        self.key.write_text('test-private-key')
        self.app = broker.create_app({'TESTING': True, 'DB_FILE': str(Path(self.tmp.name) / 'test.db'), 'KEY_FILE': str(self.key)})
        self.client = self.app.test_client()
        self.get = patch.object(broker.requests, 'get', side_effect=self.identity).start()
        self.upstream = patch.object(broker.requests, 'request', return_value=Reply({'status': True, 'data': {'file': 'private config', 'fileName': 'peer.conf'}})).start()
        self.headers = {'Authorization': 'Bearer user'}
        self.admin = {'Authorization': 'Bearer admin'}
        self.addCleanup(patch.stopall)
        self.addCleanup(self.tmp.cleanup)

    def identity(self, url, **kwargs):
        token = kwargs.get('headers', {}).get('Authorization', '')
        if url.endswith('/internal/vpn/users'):
            return Reply([{'id': USER, 'username': 'fixture', 'name': 'Fixture'}])
        if '/internal/vpn/users/' in url:
            return Reply({'id': USER})
        if token == 'Bearer admin':
            return Reply({'id': ADMIN, 'admin': True})
        if token == 'Bearer user':
            return Reply({'id': USER, 'admin': False})
        return Reply(status=401)

    def seed(self, user=USER, item='owned'):
        import sqlite3
        with sqlite3.connect(self.app.config['DB_FILE']) as conn:
            conn.execute('INSERT INTO assignments VALUES (?,?,?,?,?,?)', (item, user, 'wg0', 'peer-public-key' if item == 'owned' else 'peer-' + item, 'Laptop', ADMIN))

    def test_anonymous_and_forged_identity_are_refused(self):
        for headers in [{}, {'X-Admin': 'true', 'X-ID': USER}, {'Authorization': 'Bearer invalid'}]:
            self.assertEqual(self.client.get('/api/vpn/assignments', headers=headers).status_code, 401)

    def test_nonadmin_cannot_manage_or_open_dashboard(self):
        for method, path in [('get', '/api/vpn/catalog'), ('post', '/api/vpn/session'), ('get', '/api/vpn/assignments?all=true'), ('post', '/api/vpn/assignments'), ('delete', '/api/vpn/assignments/owned'), ('get', '/api/vpn/dashboard/')]:
            self.assertEqual(getattr(self.client, method)(path, headers=self.headers).status_code, 403)

    def test_users_list_only_owned_assignments(self):
        self.seed(); self.seed(OTHER, 'foreign')
        self.assertEqual([r['id'] for r in self.client.get('/api/vpn/assignments', headers=self.headers).json], ['owned'])

    def test_configuration_idor_is_not_forwarded(self):
        self.seed(OTHER)
        self.assertEqual(self.client.get('/api/vpn/assignments/owned/configuration', headers=self.headers).status_code, 404)
        self.upstream.assert_not_called()

    def test_owner_downloads_only_pinned_peer_with_no_cache(self):
        self.seed()
        response = self.client.get('/api/vpn/assignments/owned/configuration', headers=self.headers)
        self.assertEqual(response.json, {'file': 'private config', 'fileName': 'Laptop.conf'})
        self.assertEqual(response.headers['Cache-Control'], 'no-store')
        self.assertEqual(self.upstream.call_args.kwargs['params'], {'id': 'peer-public-key'})
        self.assertEqual(self.upstream.call_args.kwargs['headers'], {'wg-dashboard-apikey': 'test-private-key'})

    def test_assignment_requires_existing_target_and_real_peer(self):
        data = {'user_id': USER, 'configuration': 'wg0', 'peer_id': 'peer', 'label': 'Laptop'}
        self.assertEqual(self.client.post('/api/vpn/assignments', headers=self.admin, json=data).status_code, 201)
        self.assertEqual(self.client.post('/api/vpn/assignments', headers=self.admin, json=data).status_code, 409)
        self.assertTrue(any('/internal/vpn/users/' in call.args[0] for call in self.get.call_args_list))

    def test_invalid_or_missing_peer_does_not_create_assignment(self):
        self.upstream.return_value = Reply({'status': False})
        data = {'user_id': USER, 'configuration': 'wg0', 'peer_id': 'missing', 'label': 'Laptop'}
        self.assertEqual(self.client.post('/api/vpn/assignments', headers=self.admin, json=data).status_code, 409)
        self.assertEqual(self.client.get('/api/vpn/assignments?all=true', headers=self.admin).json, [])

    def test_cookie_proxy_requires_same_origin_and_live_admin(self):
        response = self.client.post('/api/vpn/session', headers=self.admin)
        cookie = response.headers['Set-Cookie']
        self.assertIn('HttpOnly', cookie); self.assertIn('SameSite=Strict', cookie)
        url = '/api/vpn/dashboard/api/validateAuthentication'
        self.assertEqual(self.client.get(url).status_code, 403)
        self.assertEqual(self.client.get(url, headers={'Origin': 'https://attacker.example'}).status_code, 403)
        self.assertEqual(self.client.get(url, headers={'Sec-Fetch-Site': 'same-origin'}).json['status'], True)
        self.get.side_effect = lambda *a, **k: Reply({'id': ADMIN, 'admin': False})
        self.assertEqual(self.client.get(url, headers={'Sec-Fetch-Site': 'same-origin'}).status_code, 403)

    def test_expired_session_is_refused(self):
        self.app.config['SESSION_TTL'] = -1
        self.client.post('/api/vpn/session', headers=self.admin)
        self.assertEqual(self.client.get('/api/vpn/dashboard/', headers={'Sec-Fetch-Site': 'same-origin'}).status_code, 401)

    def test_logout_invalidates_proxy_cookie(self):
        self.client.post('/api/vpn/session', headers=self.admin)
        self.client.delete('/api/vpn/session', headers={'Sec-Fetch-Site': 'same-origin'})
        self.assertEqual(self.client.get('/api/vpn/dashboard/', headers={'Sec-Fetch-Site': 'same-origin'}).status_code, 401)

    def test_proxy_strips_browser_credentials_and_upstream_cookies(self):
        self.upstream.return_value = Reply(headers={'Set-Cookie': 'upstream=secret', 'Access-Control-Allow-Origin': '*', 'Content-Type': 'application/json'})
        response = self.client.post('/api/vpn/dashboard/api/action', headers={**self.admin, 'Content-Type': 'application/json'}, json={'action': 'edit'})
        self.assertEqual(response.status_code, 200)
        self.assertNotIn('Set-Cookie', response.headers)
        self.assertNotIn('Access-Control-Allow-Origin', response.headers)
        headers = self.upstream.call_args.kwargs['headers']
        self.assertNotIn('Authorization', headers)
        self.assertEqual(headers['Content-Type'], 'application/json')

    def test_identity_service_failure_does_not_open_access(self):
        self.get.side_effect = requests.ConnectionError('private upstream detail')
        response = self.client.get('/api/vpn/assignments', headers=self.headers)
        self.assertEqual(response.status_code, 503)
        self.assertNotIn('private upstream detail', response.get_data(as_text=True))

    def test_user_directory_is_admin_only_and_array_shaped(self):
        self.assertEqual(self.client.get('/api/vpn/users', headers=self.headers).status_code, 403)
        response = self.client.get('/api/vpn/users?search=fixture&page=1', headers=self.admin)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json[0]['id'], USER)

    def test_unavailable_directory_never_passes_html_to_the_ui(self):
        original = self.identity
        self.get.side_effect = lambda url, **kw: Reply('html page') if url.endswith('/internal/vpn/users') else original(url, **kw)
        self.assertEqual(self.client.get('/api/vpn/users', headers=self.admin).status_code, 502)

    def test_missing_target_user_cannot_receive_a_peer(self):
        original = self.identity
        self.get.side_effect = lambda url, **kw: Reply(status=404) if '/internal/vpn/users/' in url else original(url, **kw)
        data = {'user_id': USER, 'configuration': 'wg0', 'peer_id': 'peer', 'label': 'Laptop'}
        self.assertEqual(self.client.post('/api/vpn/assignments', headers=self.admin, json=data).status_code, 400)
        self.upstream.assert_not_called()

    def test_native_rename_preserves_assignment_ownership(self):
        self.seed()
        self.client.post('/api/vpn/dashboard/api/renameWireguardConfiguration', headers=self.admin,
                         json={'ConfigurationName': 'wg0', 'NewConfigurationName': 'wg-new'})
        rows = self.client.get('/api/vpn/assignments', headers=self.headers).json
        self.assertEqual(rows[0]['configuration'], 'wg-new')
        self.assertEqual(rows[0]['user_id'], USER)

    def test_native_delete_cleans_only_the_deleted_peer_assignment(self):
        self.seed(); self.seed(OTHER, 'foreign')
        self.client.post('/api/vpn/dashboard/api/deletePeers/wg0', headers=self.admin,
                         json={'peers': ['peer-public-key']})
        rows = self.client.get('/api/vpn/assignments?all=true', headers=self.admin).json
        self.assertEqual([row['id'] for row in rows], ['foreign'])

    def test_failed_native_mutation_does_not_change_ownership(self):
        self.seed()
        self.upstream.return_value = Reply({'status': False})
        self.client.post('/api/vpn/dashboard/api/deleteWireguardConfiguration', headers=self.admin,
                         json={'ConfigurationName': 'wg0'})
        self.assertEqual(len(self.client.get('/api/vpn/assignments', headers=self.headers).json), 1)

    def test_unassign_only_removes_download_ownership(self):
        self.seed()
        self.assertEqual(self.client.delete('/api/vpn/assignments/owned', headers=self.admin).status_code, 200)
        self.upstream.assert_not_called()
        self.assertEqual(self.client.get('/api/vpn/assignments', headers=self.headers).json, [])

if __name__ == '__main__':
    unittest.main()
