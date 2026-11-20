import importlib.util
from pathlib import Path
import unittest
import json
import os
import subprocess
import tempfile
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('release', Path(__file__).with_name('release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class VersionTests(unittest.TestCase):
    def test_first_release(self):
        self.assertEqual(release.next_version(None, ['feat!: initial application']), 'v0.0.1')

    def test_conventional_commit_bumps(self):
        for messages, expected in [
            (['fix: correct footer'], 'v0.2.4'),
            (['chore(deps): update', 'docs: describe'], 'v0.2.4'),
            (['feat(portal): add search', 'fix: spacing'], 'v0.3.0'),
            (['fix!: remove setting'], 'v1.0.0'),
            (['chore: configuration\n\nBREAKING CHANGE: remove setting'], 'v1.0.0'),
            (['fix: configuration\n\nBREAKING-CHANGE: remove setting'], 'v1.0.0'),
            (['FEAT: search'], 'v0.3.0'),
        ]:
            with self.subTest(messages=messages):
                self.assertEqual(release.next_version('v0.2.3', messages), expected)

    def test_malformed_messages_fail_closed(self):
        for message in ['update code', 'fix:', 'unknown: change']:
            with self.subTest(message=message), self.assertRaises(ValueError):
                release.next_version('v0.2.3', [message])

    def test_reuses_partial_image_publication(self):
        with patch.object(release.subprocess, 'run') as inspect, patch.object(release, 'run') as command:
            inspect.return_value = subprocess.CompletedProcess([], 0, 'sha256:' + 'a' * 64, '')
            command.side_effect = ['', json.dumps([{'Config': {'Labels': {
                'org.opencontainers.image.version': 'v0.0.1',
                'org.opencontainers.image.revision': 'b' * 40}}}])]
            self.assertEqual(release.published_image('v0.0.1', 'b' * 40), 'sha256:' + 'a' * 64)

    def test_first_package_creation_can_follow_metadata_404(self):
        with patch.object(release.subprocess, 'run') as inspect:
            inspect.side_effect = [
                subprocess.CompletedProcess([], 1, '', 'failed to fetch token: 403 Forbidden'),
                subprocess.CompletedProcess([], 1, '', 'gh: Package not found (HTTP 404)'),
            ]
            self.assertEqual(release.published_image('v0.0.1', 'b' * 40), '')

    def test_registry_errors_fail_closed(self):
        with patch.object(release.subprocess, 'run') as inspect:
            inspect.return_value = subprocess.CompletedProcess([], 1, '', 'unauthorized')
            with self.assertRaises(RuntimeError):
                release.published_image('v0.0.1', 'b' * 40)


class HistoryTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.previous_directory = os.getcwd()
        os.chdir(self.directory.name)
        self.addCleanup(os.chdir, self.previous_directory)
        self.git('init', '-q')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('config', 'user.name', 'Release fixture')
        self.commit('legacy initial commit')

    def git(self, *args):
        return subprocess.check_output(['git', *args], text=True, stderr=subprocess.PIPE).strip()

    def commit(self, message):
        self.git('commit', '--allow-empty', '-qm', message)
        return self.git('rev-parse', 'HEAD')

    def test_bootstrap_and_retry(self):
        result = release.plan('HEAD')
        self.assertEqual(result['version'], 'v0.0.1')
        self.assertEqual(result['existing'], 'false')
        self.git('tag', result['version'])
        retry = release.plan('HEAD')
        self.assertEqual(retry['version'], result['version'])
        self.assertEqual(retry['existing'], 'true')

    def test_unreleased_history_uses_highest_bump(self):
        self.git('tag', 'v0.0.1')
        self.commit('feat(portal): search')
        self.commit('fix: alignment')
        self.assertEqual(release.plan('HEAD')['version'], 'v0.1.0')

    def test_stale_revision_cannot_release(self):
        old = self.git('rev-parse', 'HEAD')
        self.commit('fix: newer')
        self.git('tag', 'v0.0.1')
        with self.assertRaises(subprocess.CalledProcessError):
            release.plan(old)

    def test_conflicting_tag_prevents_external_writes(self):
        self.git('tag', 'v0.0.1')
        newer = self.commit('fix: newer')
        with patch.object(release, 'run', wraps=release.run) as calls:
            with self.assertRaises(ValueError):
                release.finalize('v0.0.1', newer, 'sha256:' + 'a' * 64, 'fixture/repo')
            self.assertFalse(any(call.args[0] == 'gh' for call in calls.call_args_list))

    def test_later_merge_recovers_image_pushed_before_tag(self):
        earlier = self.git('rev-parse', 'HEAD')
        target = self.commit('fix: later change')
        with patch.object(release, 'run', wraps=release.run) as calls, patch.object(release, 'published_image') as image, patch.object(release, 'finalize') as finish:
            def command(*args):
                if args[0] == 'gh':
                    return '[[]]'
                return subprocess.check_output(args, text=True).strip()
            calls.side_effect = command
            image.side_effect = [release.PublishedImageConflict(earlier, 'sha256:' + 'a' * 64), '']
            def completed(version, revision, digest, repository):
                self.git('tag', version, revision)
            finish.side_effect = completed
            result = release.prepare(target, 'fixture/repo')
            self.assertEqual(result['version'], 'v0.0.2')
            self.assertEqual(result['digest'], '')
            finish.assert_called_once_with('v0.0.1', earlier, 'sha256:' + 'a' * 64, 'fixture/repo')

    def test_later_merge_completes_tag_without_release(self):
        earlier = self.git('rev-parse', 'HEAD')
        self.git('tag', 'v0.0.1')
        target = self.commit('fix: later change')
        with patch.object(release, 'run', wraps=release.run) as calls, patch.object(release, 'published_image') as image, patch.object(release, 'finalize') as finish:
            def command(*args):
                if args[0] == 'gh':
                    return '[[]]'
                return subprocess.check_output(args, text=True).strip()
            calls.side_effect = command
            image.side_effect = ['sha256:' + 'a' * 64, '']
            result = release.prepare(target, 'fixture/repo')
            self.assertEqual(result['version'], 'v0.0.2')
            finish.assert_called_once_with('v0.0.1', earlier, 'sha256:' + 'a' * 64, 'fixture/repo')

    def test_invalid_digest_prevents_external_writes(self):
        with self.assertRaises(ValueError):
            release.finalize('v0.0.1', self.git('rev-parse', 'HEAD'), 'unknown', 'fixture/repo')


if __name__ == '__main__':
    unittest.main()
