"""Prepare private WGDashboard state, preserving existing configuration."""
import configparser
import os
import secrets
from pathlib import Path
import bcrypt

os.umask(0o077)
path = Path('/data/wg-dashboard.ini')
config = configparser.RawConfigParser()
config.read(path)
for section in ['Server', 'Account', 'Other', 'WireGuardConfiguration', 'Clients', 'Peers']:
    if not config.has_section(section):
        config.add_section(section)
config.set('Server', 'app_prefix', '/api/vpn/dashboard')
config.set('Server', 'dashboard_api_key', 'true')
config.set('Server', 'auth_req', 'true')
config.set('Server', 'app_port', '10086')
config.set('Account', 'username', config.get('Account', 'username', fallback='cloudmatehub-service'))
if not config.has_option('Account', 'password'):
    config.set('Account', 'password', bcrypt.hashpw(secrets.token_bytes(48), bcrypt.gensalt()).decode())
config.set('Other', 'welcome_session', 'false')
if not config.has_option('WireGuardConfiguration', 'autostart'):
    config.set('WireGuardConfiguration', 'autostart', '')
if not config.has_option('Clients', 'sign_up'):
    config.set('Clients', 'sign_up', 'false')
config.set('Peers', 'remote_endpoint', config.get('Peers', 'remote_endpoint', fallback='localhost'))
with path.open('w') as file:
    config.write(file)
