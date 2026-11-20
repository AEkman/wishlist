"""Plan and finish validated Wishlist releases (CI tooling only)."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess

VERSION = re.compile(r'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z')
COMMIT = re.compile(r'(feat|fix|perf|refactor|docs|test|build|ci|chore|revert)(?:\([^()\n]+\))?(!)?: \S.*', re.I)
IMAGE = 'ghcr.io/aekman/wishlist'


def next_version(previous, messages):
    # Bootstrap includes legacy history which predates the commit-title policy.
    if previous is None:
        return 'v0.0.1'
    match = VERSION.fullmatch(previous)
    if not match:
        raise ValueError('invalid previous release version')
    bump = 0
    for message in messages:
        header = COMMIT.fullmatch(message.splitlines()[0]) if message else None
        if not header:
            raise ValueError(f'Not an accepted Conventional Commit: {message!r}')
        if header[2] or re.search(r'^BREAKING[ -]CHANGE: \S', message, re.M):
            bump = max(bump, 3)
        elif header[1].lower() == 'feat':
            bump = max(bump, 2)
        else:
            bump = max(bump, 1)
    if not bump:
        return previous
    major, minor, patch = map(int, match.groups())
    return f'v{major + 1}.0.0' if bump == 3 else f'v{major}.{minor + 1}.0' if bump == 2 else f'v{major}.{minor}.{patch + 1}'


def run(*args):
    return subprocess.check_output(args, text=True).strip()


def git(*args):
    return run('git', *args)


def plan(target):
    target = git('rev-parse', target + '^{commit}')
    tags = [tag for tag in git('tag', '--list', 'v*').splitlines() if VERSION.fullmatch(tag)]
    tags.sort(key=lambda tag: tuple(map(int, VERSION.fullmatch(tag).groups())))
    previous = tags[-1] if tags else None
    if previous:
        revision = git('rev-parse', previous + '^{commit}')
        if revision == target:
            return {'version': previous, 'revision': target, 'existing': 'true'}
        # An older job must never republish main or release an older ancestor.
        subprocess.run(['git', 'merge-base', '--is-ancestor', revision, target], check=True)
    history = git('log', '--format=%B%x00', f'{previous}..{target}' if previous else target)
    messages = [message.strip() for message in history.split('\0') if message.strip()]
    return {'version': next_version(previous, messages), 'revision': target, 'existing': 'false'}


class PublishedImageConflict(ValueError):
    def __init__(self, revision, digest):
        super().__init__('existing versioned image has a different release identity')
        self.revision = revision
        self.digest = digest


def prepare(target, repository):
    target = git('rev-parse', target + '^{commit}')
    pages = json.loads(run('gh', 'api', '--paginate', '--slurp', f'repos/{repository}/releases'))
    released = {item['tag_name'] for page in pages for item in page if not item['draft']}
    for tag in git('tag', '--list', 'v*').splitlines():
        if not VERSION.fullmatch(tag) or tag in released:
            continue
        revision = git('rev-parse', tag + '^{commit}')
        subprocess.run(['git', 'merge-base', '--is-ancestor', revision, target], check=True)
        digest = published_image(tag, revision)
        if not digest:
            raise ValueError('release tag exists without its published image')
        finalize(tag, revision, digest, repository)
    recovered = set()
    while True:
        result = plan(target)
        try:
            result['digest'] = published_image(result['version'], target)
            return result
        except PublishedImageConflict as conflict:
            # A prior run pushed immutable bytes but stopped before creating its tag.
            if result['version'] in recovered:
                raise ValueError('release recovery made no progress')
            recovered.add(result['version'])
            subprocess.run(['git', 'merge-base', '--is-ancestor', conflict.revision, target], check=True)
            finalize(result['version'], conflict.revision, conflict.digest, repository)


def published_image(version, revision):
    reference = f'{IMAGE}:{version}'
    result = subprocess.run(['docker', 'buildx', 'imagetools', 'inspect', reference,
                             '--format', '{{.Manifest.Digest}}'], capture_output=True, text=True)
    if result.returncode:
        if 'not found' in result.stderr.lower() or 'manifest unknown' in result.stderr.lower():
            return ''
        # GHCR may reject the pull-token request before a package exists.
        # A metadata 404 permits a creation attempt. It can also hide a private
        # package; the subsequent push still enforces namespace permissions.
        owner, package = IMAGE.removeprefix('ghcr.io/').split('/', 1)
        package_status = subprocess.run(
            ['gh', 'api', f'users/{owner}/packages/container/{package}', '--silent'],
            capture_output=True, text=True)
        if package_status.returncode and '(HTTP 404)' in package_status.stderr:
            return ''
        raise RuntimeError(result.stderr)
    digest = result.stdout.strip()
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', digest):
        raise ValueError('registry returned an invalid image digest')
    run('docker', 'pull', '--platform', 'linux/amd64', f'{IMAGE}@{digest}')
    config = json.loads(run('docker', 'image', 'inspect', f'{IMAGE}@{digest}'))[0]
    labels = config['Config']['Labels']
    if labels.get('org.opencontainers.image.version') != version:
        raise ValueError('existing image has a different version')
    image_revision = labels.get('org.opencontainers.image.revision', '')
    if not re.fullmatch(r'[0-9a-f]{40}', image_revision):
        raise ValueError('existing image has an invalid source revision')
    if image_revision != revision:
        raise PublishedImageConflict(image_revision, digest)
    return digest


def finalize(version, revision, digest, repository):
    if not VERSION.fullmatch(version) or not re.fullmatch(r'[0-9a-f]{40}', revision):
        raise ValueError('invalid release identity')
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', digest):
        raise ValueError('invalid image digest')
    tags = git('tag', '--list', version)
    if tags:
        if git('rev-parse', version + '^{commit}') != revision:
            raise ValueError('release tag already points to different code')
    else:
        run('gh', 'api', f'repos/{repository}/git/refs', '-f', f'ref=refs/tags/{version}', '-f', f'sha={revision}')
        git('update-ref', f'refs/tags/{version}', revision)
    # Retry a partial publication by completing its missing release, never moving tags.
    existing = subprocess.run(['gh', 'release', 'view', version, '--repo', repository], capture_output=True)
    if existing.returncode == 0:
        return
    notes = Path(os.environ.get('RUNNER_TEMP', '/tmp')) / 'wishlist-release-notes.md'
    notes.write_text(f'Source revision: `{revision}`\n\nImage: `{IMAGE}:{version}`\n\nExact image: `{IMAGE}@{digest}`\n')
    run('gh', 'release', 'create', version, '--repo', repository, '--verify-tag', '--title', version,
        '--notes-file', str(notes), '--generate-notes')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['plan', 'prepare', 'finalize'])
    args = parser.parse_args()
    if args.command in ('plan', 'prepare'):
        result = (prepare(os.environ['GITHUB_SHA'], os.environ['GITHUB_REPOSITORY'])
                  if args.command == 'prepare' else plan(os.environ['GITHUB_SHA']))
        result['image_version'] = result['version'].removeprefix('v')
        print(json.dumps(result))
        with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
            output.write(''.join(f'{key}={value}\n' for key, value in result.items()))
    else:
        finalize(os.environ['RELEASE_VERSION'], os.environ['GITHUB_SHA'],
                 os.environ['IMAGE_DIGEST'], os.environ['GITHUB_REPOSITORY'])


if __name__ == '__main__':
    main()
