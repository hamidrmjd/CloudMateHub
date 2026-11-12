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
VERSION_TAG = r'v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('namespace', help='Docker Hub username or organization')
    parser.add_argument('--tag', required=True, help='Immutable release tag')
    parser.add_argument('--platforms', default='linux/amd64,linux/arm64')
    parser.add_argument('--dry-run', action='store_true')
    args = parser.parse_args()
    if not re.fullmatch(r'[a-z0-9][a-z0-9_-]*', args.namespace):
        parser.error('invalid Docker Hub namespace')
    if len(args.tag) > 128 or not re.fullmatch(VERSION_TAG, args.tag):
        parser.error('release tag must use vMAJOR.MINOR.PATCH')
    version = re.search(r'^CLOUDMATEHUB_VERSION=(.+)$', (ROOT / '.env').read_text(), re.MULTILINE)
    if not version or args.tag != f'v{version[1]}':
        parser.error('release tag must match CLOUDMATEHUB_VERSION in .env')
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
        target['tags'] = [f'{args.namespace}/cloudmatehub-{service}:{args.tag}']
        targets[service] = target
    if args.dry_run:
        print(json.dumps({s: t['tags'] for s, t in targets.items()}, indent=2))
        return
    if subprocess.run(['git', 'diff', '--quiet', 'HEAD', '--'], cwd=ROOT).returncode:
        parser.error('commit tracked changes before publishing')
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    for target in targets.values():
        tag = target['tags'][0]
        result = subprocess.run(['docker', 'buildx', 'imagetools', 'inspect', tag],
                                cwd=ROOT, capture_output=True, text=True)
        if result.returncode == 0:
            parser.error(f'image already exists: {tag}')
        if not any(error in result.stderr.lower() for error in ('not found', 'manifest unknown')):
            parser.error(f'could not check {tag}: {result.stderr.strip()}')
        target['labels'] = {'org.opencontainers.image.version': args.tag,
                            'org.opencontainers.image.revision': revision}
    with tempfile.TemporaryDirectory(prefix='cloudmatehub-publish-') as directory:
        path = Path(directory) / 'bake.json'
        path.write_text(json.dumps({'target': targets}))
        path.chmod(0o600)
        # Sequential targets reduce registry pressure; Docker reuses shared layers.
        for service in SERVICES:
            subprocess.run(['docker', 'buildx', 'bake', '-f', str(path), service, '--push'],
                           cwd=ROOT, check=True)
        for target in targets.values():
            tag = target['tags'][0]
            manifest = json.loads(subprocess.check_output(
                ['docker', 'buildx', 'imagetools', 'inspect', '--raw', tag], cwd=ROOT, text=True))
            published = {f"{platform.get('os')}/{platform.get('architecture')}"
                         for entry in manifest.get('manifests', [])
                         if (platform := entry.get('platform'))}
            if not set(platforms).issubset(published):
                parser.error(f'{tag} lacks requested platforms: {sorted(set(platforms) - published)}')


if __name__ == '__main__':
    main()
