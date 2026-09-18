import json
import tempfile
import unittest
from pathlib import Path
from release import proposal, update_versions, validate_version


class ReleaseTests(unittest.TestCase):
    def test_first_release_and_numeric_order(self):
        self.assertEqual(proposal([], '0.1.0'), '0.1.0')
        self.assertEqual(proposal(['v0.9.9', 'v0.10.2', 'sdk/v9.9.9'], '0.1.0'), '0.10.3')

    def test_immutable_semver(self):
        for version in ['0.1.0', '0.0.9', 'v0.2.0', '2.0.0', '0.01.0', '0.2.0-rc1', 'x']:
            with self.subTest(version=version), self.assertRaises(ValueError):
                validate_version(version, ['v0.1.0'])
        validate_version('1.0.0', ['v0.1.0'])

    def test_all_version_inputs_move_together(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ('sdk/ts/package.json', 'web/package.json'):
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(json.dumps({'version':'0.1.0','dependencies':{'@gopherex/courier-sdk':'0.1.0'}}))
            (root / 'openapi').mkdir()
            (root / 'openapi/openapi.yaml').write_text('openapi: 3.0.3\ninfo:\n  version: 0.1.0\n')
            update_versions(root, '0.2.0')
            sdk = json.loads((root / 'sdk/ts/package.json').read_text())
            web = json.loads((root / 'web/package.json').read_text())
            self.assertEqual(sdk['version'], '0.2.0')
            self.assertEqual(web['dependencies']['@gopherex/courier-sdk'], sdk['version'])
            self.assertEqual(web['version'], sdk['version'])
            self.assertIn('version: 0.2.0', (root / 'openapi/openapi.yaml').read_text())


if __name__ == '__main__':
    unittest.main()
