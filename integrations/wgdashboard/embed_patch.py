"""Adapt the pinned native frontend to Hub-owned navigation and sign-in."""
from pathlib import Path
import re


def patch(root):
    path = next(Path(root).glob('DashboardConfigurationStore-*.js'))
    source = path.read_text()
    # Native credentials are never requested inside Hub. Authentication failures
    # return to the parent, which can renew its authenticated broker session.
    notify = 'window.parent.postMessage({type:`cmh-vpn-session-error`},window.location.origin)'
    source, count = re.subn(r'\{path:`/signin`,component:.*?meta:\{title:`Sign In`,hideTopNav:!0\}\}',
                            '{path:`/signin`,redirect:`/`}', source)
    if count != 1:
        raise RuntimeError('Pinned WGDashboard sign-in route changed')
    if 'n(`/signin`)' not in source or 'X.push({path:`/signin`})' not in source:
        raise RuntimeError('Pinned WGDashboard authentication handling changed')
    source = source.replace('n(`/signin`)', 'n(!1),' + notify)
    source = source.replace('X.push({path:`/signin`})', notify)
    path.write_text(source)


if __name__ == '__main__':
    patch('/opt/wgdashboard/src/static/dist/WGDashboardAdmin/assets')
