#!/usr/bin/env python3
"""Interactive, immutable tag release. --plan never mutates the repository."""
import argparse
import json
import re
import subprocess
from pathlib import Path


def run(*args, capture=False):
    result = subprocess.run(args, check=True, text=True, capture_output=capture)
    return result.stdout.strip() if capture else None


def version_tuple(value):
    if not re.fullmatch(r"(?:0|1)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", value):
        raise ValueError("Use MAJOR.MINOR.PATCH with major 0 or 1; v2 requires a new Go module path")
    return tuple(map(int, value.split('.')))


def proposal(tags, package_version):
    versions = [tuple(map(int, tag[1:].split('.'))) for tag in tags
                if re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag)]
    if not versions:
        version_tuple(package_version)
        return package_version
    major, minor, patch = max(versions)
    return f"{major}.{minor}.{patch + 1}"


def validate_version(version, tags):
    parsed = version_tuple(version)
    published = [tuple(map(int, tag[1:].split('.'))) for tag in tags
                 if re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag)]
    if published and parsed <= max(published):
        raise ValueError("Version must be newer than every published tag; tags are immutable")


def update_versions(root, version):
    version_tuple(version)
    for name in ('sdk/ts/package.json', 'web/package.json'):
        path = root / name
        package = json.loads(path.read_text())
        package['version'] = version
        if name == 'web/package.json':
            package['dependencies']['@gopherex/courier-sdk'] = version
        path.write_text(json.dumps(package, indent=2) + '\n')
    path = root / 'openapi/openapi.yaml'
    text, count = re.subn(r'(?m)^  version: [^\n]+$', f'  version: {version}', path.read_text(), count=1)
    if count != 1:
        raise ValueError('OpenAPI info.version not found')
    path.write_text(text)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--plan', action='store_true')
    parser.add_argument('--version')
    args = parser.parse_args()
    root = Path(run('git', 'rev-parse', '--show-toplevel', capture=True))
    if Path.cwd() != root:
        raise ValueError('Run from repository root')
    if not args.plan:
        if run('git', 'status', '--porcelain', capture=True):
            raise ValueError('Commit all changes before releasing')
        if run('git', 'branch', '--show-current', capture=True) != 'master':
            raise ValueError('Release from master')
        run('git', 'fetch', 'origin', '--tags')
        if run('git', 'rev-parse', 'HEAD', capture=True) != run('git', 'rev-parse', 'origin/master', capture=True):
            raise ValueError('HEAD must match origin/master; push or update the branch first')
    tags = run('git', 'tag', '--list', capture=True).splitlines()
    package = json.loads((root / 'sdk/ts/package.json').read_text())
    version = args.version or proposal(tags, package['version'])
    if not args.plan and not args.version:
        version = input(f'Release version [{version}]: ').strip() or version
    validate_version(version, tags)
    print(f'Release v{version}: update versions, regenerate, check, commit, tag and atomic push')
    print(f'CI publishes ghcr.io/gopherex/courier:{version}, GitHub npm package and release assets')
    if args.plan:
        return
    if input("Type 'yes' to proceed: ").strip() != 'yes':
        print('Cancelled')
        return
    update_versions(root, version)
    run('make', 'node-deps', 'generate', 'check')
    run('git', 'add', 'sdk/ts/package.json', 'web/package.json', 'openapi/openapi.yaml',
        'pkg/api', 'sdk/ts/src/gen', 'internal/postgres/gen', 'yarn.lock')
    if run('git', 'diff', '--cached', '--name-only', capture=True):
        run('git', 'commit', '-m', f'chore(release): prepare v{version}')
    if run('git', 'status', '--porcelain', capture=True):
        raise ValueError('Unexpected remaining changes; inspect before tagging')
    run('git', 'tag', '-a', f'v{version}', '-m', f'v{version}')
    run('git', 'push', '--atomic', 'origin', 'HEAD:refs/heads/master', f'refs/tags/v{version}')
    print('Tag pushed. Follow the Release workflow; publication completes asynchronously.')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, subprocess.CalledProcessError, EOFError) as error:
        raise SystemExit(str(error)) from error
