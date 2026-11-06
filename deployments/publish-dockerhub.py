#!/usr/bin/env python3
"""Build and publish CloudMateHub images using Docker's existing credential store."""
import argparse
import json
from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent.parent
SERVICES = ('server', 'ui', 'gateway', 'https-manager', 'wgdashboard')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('namespace', help='Docker Hub username or organization')
    parser.add_argument('--tag', required=True, help='Immutable release tag')
    parser.add_argument('--platforms', default='linux/amd64,linux/arm64')
    parser.add_argument('--dry-run', action='store_true')
    args = parser.parse_args()
    if not re.fullmatch(r'[a-z0-9][a-z0-9_-]*', args.namespace):
        parser.error('invalid Docker Hub namespace')
    if not re.fullmatch(r'[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}', args.tag):
        parser.error('invalid release tag')
    platforms = args.platforms.split(',')
    if not platforms or any(p not in ('linux/amd64', 'linux/arm64') for p in platforms):
        parser.error('supported platforms: linux/amd64,linux/arm64')
    # Keep runtime configuration in memory. Only source-build arguments go into Bake.
    config = json.loads(subprocess.check_output(
        [str(ROOT / 'bin/docker-compose'), 'config', '--format', 'json'], cwd=ROOT, text=True))
    targets = {}
    for service in SERVICES:
        build = config['services'][service].get('build')
        if not build:
            parser.error('disable the pull-only image override before building source images')
        target = {k: build[k] for k in ('context', 'dockerfile', 'args', 'target') if k in build}
        target['platforms'] = platforms
        target['tags'] = list(dict.fromkeys(
            f'{args.namespace}/cloudmatehub-{service}:{tag}' for tag in (args.tag, 'latest')))
        targets[service] = target
    if args.dry_run:
        print(json.dumps({s: t['tags'] for s, t in targets.items()}, indent=2))
        return
    with tempfile.TemporaryDirectory(prefix='cloudmatehub-publish-') as directory:
        path = Path(directory) / 'bake.json'
        path.write_text(json.dumps({'target': targets}))
        path.chmod(0o600)
        # Sequential targets reduce registry pressure; Docker reuses shared layers.
        for service in SERVICES:
            subprocess.run(['docker', 'buildx', 'bake', '-f', str(path), service, '--push'],
                           cwd=ROOT, check=True)


if __name__ == '__main__':
    main()
