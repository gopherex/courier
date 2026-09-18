"""Exercise the actual Makefile recipe; Git operations stay in temporary repositories."""
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

MAKEFILE = Path(__file__).resolve().parent.parent / 'Makefile'


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        base = Path(self.temp.name)
        self.repo = base / 'checkout'
        self.repo.mkdir()
        self.remote = base / 'remote.git'
        self.git('init', '--bare', str(self.remote))
        self.git('init', '-b', 'master')
        self.git('config', 'user.name', 'Release Test')
        self.git('config', 'user.email', 'release@example.test')
        self.git('config', 'commit.gpgsign', 'false')
        self.git('config', 'tag.gpgsign', 'false')
        self.git('config', 'core.hooksPath', str(base / 'no-hooks'))
        shutil.copyfile(MAKEFILE, self.repo / 'Makefile')
        for name in ('sdk/ts/package.json', 'web/package.json'):
            path = self.repo / name
            path.parent.mkdir(parents=True)
            path.write_text(json.dumps({'version':'0.1.0','dependencies':{'@gopherex/courier-sdk':'0.1.0'}}))
        self.git('add', '.')
        self.git('commit', '-m', 'initial')
        self.git('remote', 'add', 'origin', str(self.remote))
        self.git('push', '-u', 'origin', 'master')
        # Dependency installation is not the subject here. Node and Git are real.
        tools = base / 'tools'
        tools.mkdir()
        yarn = tools / 'yarn'
        yarn.write_text('#!/bin/sh\nexit 0\n')
        yarn.chmod(0o755)
        self.env = {**os.environ, 'PATH':str(tools)+os.pathsep+os.environ['PATH'],
                    'GIT_TERMINAL_PROMPT':'0'}

    def git(self, *args):
        return subprocess.check_output(['git', *args], cwd=self.repo, text=True,
                                       stderr=subprocess.PIPE).strip()

    def release(self, answers, succeeds=True):
        result = subprocess.run(['make', 'release'], input=answers, cwd=self.repo,
                                env=self.env, text=True, capture_output=True, timeout=30)
        if succeeds:
            self.assertEqual(result.returncode, 0, result.stdout+result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout+result.stderr)
        return result.stdout+result.stderr

    def tag(self, version):
        self.git('tag', '-a', 'v'+version, '-m', 'v'+version)
        self.git('push', 'origin', 'v'+version)

    def assert_bump(self, choice, expected):
        self.tag('0.1.0')
        output = self.release('1\n'+choice+'\nyes\n')
        self.assertIn('Released v'+expected, output)
        sdk = json.loads((self.repo / 'sdk/ts/package.json').read_text())
        web = json.loads((self.repo / 'web/package.json').read_text())
        self.assertEqual(sdk['version'], expected)
        self.assertEqual(web['dependencies']['@gopherex/courier-sdk'], expected)
        self.assertEqual(self.git('cat-file', '-t', 'v'+expected), 'tag')
        self.assertEqual(self.git('rev-parse', 'HEAD'), self.git('rev-parse', 'v'+expected+'^{}'))
        self.assertEqual(self.git('rev-parse', 'HEAD'), self.git('rev-parse', 'origin/master'))
        self.assertEqual(self.git('status', '--porcelain'), '')
        self.assertTrue(self.git('ls-remote', '--tags', 'origin', 'refs/tags/v'+expected))

    def test_major(self):
        self.assert_bump('1', '1.0.0')

    def test_minor(self):
        self.assert_bump('2', '0.2.0')

    def test_patch(self):
        self.assert_bump('3', '0.1.1')

    def test_first_release_starts_at_zero(self):
        output = self.release('1\n3\nyes\n')
        self.assertIn('Latest release: v0.0.0', output)
        self.assertIn('Released v0.0.1', output)

    def test_numeric_tag_order(self):
        self.tag('0.9.9')
        self.tag('0.10.2')
        self.assertIn('Released v0.10.3', self.release('1\n3\nyes\n'))

    def test_recreate_local_tag_missing_on_remote(self):
        self.git('tag', '-a', 'v0.1.0', '-m', 'v0.1.0')
        self.assertIn('Recreated v0.1.0', self.release('2\nyes\n'))
        self.assertTrue(self.git('ls-remote', '--tags', 'origin', 'refs/tags/v0.1.0'))

    def test_cancel_and_unconfirmed_bump(self):
        head = self.git('rev-parse', 'HEAD')
        self.assertIn('3) cancel', self.release('3\n'))
        self.assertIn('Aborted.', self.release('1\n2\nno\n'))
        self.assertEqual(self.git('rev-parse', 'HEAD'), head)
        self.assertEqual(self.git('tag', '--list'), '')
        self.assertEqual(self.git('status', '--porcelain'), '')

    def test_dirty_tree_rejected(self):
        (self.repo / 'uncommitted').write_text('change')
        self.assertIn('Working tree is not clean', self.release('3\n', succeeds=False))

    def test_no_tag_to_recreate(self):
        self.assertIn('No release tag to recreate', self.release('2\n', succeeds=False))

    def test_mismatched_sdk_rejected(self):
        self.tag('0.2.0')
        self.assertIn('does not match', self.release('2\nyes\n', succeeds=False))

    def test_v2_rejected(self):
        self.tag('1.0.0')
        self.assertIn('semantic import versioning', self.release('1\n1\n', succeeds=False))

    def test_recreate_on_local_head(self):
        self.tag('0.1.0')
        old = self.git('rev-parse', 'HEAD')
        (self.repo / 'change').write_text('new HEAD')
        self.git('add', '.')
        self.git('commit', '-m', 'change')
        head = self.git('rev-parse', 'HEAD')
        self.assertIn('Cancelled.', self.release('3\n'))
        self.assertIn('Aborted.', self.release('2\nno\n'))
        self.assertEqual(self.git('rev-parse', 'v0.1.0^{}'), old)
        self.assertIn('Recreated v0.1.0', self.release('2\nyes\n'))
        self.assertEqual(self.git('rev-parse', 'v0.1.0^{}'), head)
        self.assertIn(head, self.git('ls-remote', '--tags', 'origin', 'refs/tags/v0.1.0^{}'))
        self.assertEqual(self.git('rev-parse', 'origin/master'), old)


if __name__ == '__main__':
    unittest.main()
