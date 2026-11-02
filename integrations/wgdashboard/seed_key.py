"""Loaded after upstream initializes; install the broker-only key without logging it."""
from pathlib import Path
import dashboard
config = dashboard.DashboardConfig
key = Path('/run/secrets/wgdashboard_key').read_text().strip()
with config.engine.begin() as conn:
    existing = conn.execute(config.apiKeyTable.select().where(config.apiKeyTable.c.Key == key)).first()
    if existing is None:
        conn.execute(config.apiKeyTable.insert().values(Key=key, ExpiredAt=None))
config.DashboardAPIKeys = config._DashboardConfig__getAPIKeys()
