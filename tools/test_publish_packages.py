import csv
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "tools/publish-packages.sh"
MANIFEST = ROOT / "setup/packages.csv"
AZURE_LINUX_4_REPOS = [
    "azurelinux-4.0-prod-microsoft-x86_64-yum",
    "azurelinux-4.0-prod-microsoft-aarch64-yum",
]


class PublishPackagesTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.artifacts = self.directory / "signed"
        (self.artifacts / "mariner").mkdir(parents=True)
        self.log = self.directory / "pmc.jsonl"
        mock = self.directory / "pmc"
        mock.write_text(
            "#!/usr/bin/env python3\n"
            "import json, os, sys\n"
            "args = sys.argv[1:]\n"
            "with open(os.environ['PMC_LOG'], 'a') as log:\n"
            "    log.write(json.dumps(args) + '\\n')\n"
            "if os.environ.get('PMC_FAIL') in args:\n"
            "    sys.exit(1)\n"
            "if 'upload' in args and not os.environ.get('PMC_EMPTY_ID'):\n"
            "    print('id-' + os.path.basename(args[-1]))\n"
        )
        mock.chmod(0o755)
        self.env = {
            **os.environ,
            "PATH": str(self.directory) + os.pathsep + os.environ["PATH"],
            "PMC_LOG": str(self.log),
        }
        self.rows = list(csv.reader(
            line for line in MANIFEST.read_text().splitlines()
            if line and not line.startswith("#")
        ))

    def artifact(self, suffix, mariner=False, version="2.5.6"):
        path = self.artifacts / ("mariner" if mariner else "")
        file = path / f"blobfuse2-{version}-{suffix}"
        file.touch()
        return file

    def azure_artifacts(self, version="2.5.6"):
        for arch in ("x86_64", "aarch64"):
            self.artifact(f"fuse3.{arch}.rpm", mariner=True, version=version)

    def all_artifacts(self, version="2.5.6"):
        self.azure_artifacts(version)
        for suffix in (
            "fuse2.x86_64.deb", "fuse3.x86_64.deb", "fuse3.arm64.deb",
            "fuse3-4.x86_64.deb", "fuse3-4.arm64.deb",
            "fuse3.x86_64.rpm", "fuse3.aarch64.rpm",
        ):
            self.artifact(suffix, version=version)

    def run_publish(self, scope="azurelinux4", manifest=MANIFEST):
        return subprocess.run(
            ["bash", str(SCRIPT), str(manifest), str(self.artifacts), scope],
            env=self.env, text=True, capture_output=True, check=False,
        )

    def calls(self):
        if not self.log.exists():
            return []
        return [json.loads(line) for line in self.log.read_text().splitlines()]

    def manifest_with_repo(self, distro, repo):
        manifest = self.directory / "packages.csv"
        with manifest.open("w", newline="") as file:
            csv.writer(file, lineterminator="\n").writerows(
                [row[0], row[1], repo, row[3]] if row[0] == distro else row
                for row in self.rows
            )
        return manifest

    def test_only_azurelinux4_uploads_and_publishes_both_architectures(self):
        self.azure_artifacts()
        result = self.run_publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        self.assertEqual(len(calls), 6)
        uploads = [call[-1] for call in calls if "upload" in call]
        self.assertEqual(len(uploads), 2)
        self.assertTrue(all("/mariner/" in path for path in uploads))
        destinations = [call[-1] for call in calls if "repo" in call]
        self.assertEqual(destinations, AZURE_LINUX_4_REPOS * 2)

    def test_all_preserves_existing_destinations_and_adds_azurelinux4(self):
        self.all_artifacts()
        result = self.run_publish("all")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        self.assertEqual(len([call for call in calls if "upload" in call]), 9)
        updates = [call for call in calls if "update" in call]
        self.assertEqual(len(updates), len(self.rows))
        expected = {row[2] for row in self.rows}
        published = [call[-1] for call in calls if "publish" in call]
        self.assertCountEqual(published, expected)
        jammy = [call for call in updates if "microsoft-ubuntu-jammy-prod-apt" in call]
        self.assertEqual(len(jammy), 2)
        self.assertTrue(all(call[-1] == "jammy" for call in jammy))

    def test_missing_repository_fails_before_any_upload(self):
        self.azure_artifacts()
        manifest = self.manifest_with_repo("AzureLinux-4.0-aarch64", "")
        result = self.run_publish(manifest=manifest)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Missing or invalid PMC repository", result.stderr)
        self.assertEqual(self.calls(), [])

    def test_unresolved_repository_placeholder_fails_before_any_upload(self):
        self.azure_artifacts()
        manifest = self.manifest_with_repo(
            "AzureLinux-4.0-x86_64", "@AZURE_LINUX_4_X86_64_REPO@"
        )
        self.assertNotEqual(self.run_publish(manifest=manifest).returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_existing_distribution_repository_is_rejected(self):
        self.azure_artifacts()
        manifest = self.manifest_with_repo(
            "AzureLinux-4.0-x86_64", "azurelinux-3.0-prod-ms-oss-x86_64-yum"
        )
        result = self.run_publish(manifest=manifest)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("must not target an existing distribution", result.stderr)
        self.assertEqual(self.calls(), [])

    def test_missing_artifact_fails_before_any_upload(self):
        self.artifact("fuse3.x86_64.rpm", mariner=True)
        result = self.run_publish()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Expected exactly one signed artifact", result.stderr)
        self.assertEqual(self.calls(), [])

    def test_multiple_versions_fail_before_any_upload(self):
        self.azure_artifacts()
        self.azure_artifacts("2.5.7")
        self.assertNotEqual(self.run_publish().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_upload_failure_stops_without_publishing(self):
        self.azure_artifacts()
        self.env["PMC_FAIL"] = "upload"
        self.assertNotEqual(self.run_publish().returncode, 0)
        self.assertEqual(len(self.calls()), 1)

    def test_empty_package_id_stops_without_repository_changes(self):
        self.azure_artifacts()
        self.env["PMC_EMPTY_ID"] = "1"
        result = self.run_publish()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("PMC returned no package ID", result.stderr)
        self.assertEqual(len(self.calls()), 1)

    def test_repository_update_failure_prevents_publishing(self):
        self.azure_artifacts()
        self.env["PMC_FAIL"] = "update"
        self.assertNotEqual(self.run_publish().returncode, 0)
        self.assertFalse(any("publish" in call for call in self.calls()))

    def test_repository_publish_failure_is_reported(self):
        self.azure_artifacts()
        self.env["PMC_FAIL"] = "publish"
        self.assertNotEqual(self.run_publish().returncode, 0)
        self.assertEqual(len(self.calls()), 5)

    def test_preview_routing_for_azurelinux3_is_preserved(self):
        self.all_artifacts("2.5.7~preview.1")
        manifest = self.directory / "azurelinux3.csv"
        with manifest.open("w", newline="") as file:
            csv.writer(file, lineterminator="\n").writerows(
                row for row in self.rows if not row[0].startswith("AzureLinux-4.0-")
            )
        result = self.run_publish("all", manifest=manifest)
        self.assertEqual(result.returncode, 0, result.stderr)
        published = [call[-1] for call in self.calls() if "publish" in call]
        self.assertIn("azurelinux-3.0-preview-ms-oss-x86_64-yum", published)
        self.assertIn("azurelinux-3.0-preview-ms-oss-aarch64-yum", published)
        self.assertNotIn("azurelinux-3.0-prod-ms-oss-x86_64-yum", published)
        self.assertTrue(all(repo not in published for repo in AZURE_LINUX_4_REPOS))

    def test_preview_is_blocked_from_azurelinux4_production(self):
        self.all_artifacts("2.5.7~preview.1")
        for scope in ("all", "azurelinux4"):
            with self.subTest(scope=scope):
                result = self.run_publish(scope)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Refusing to publish a preview package", result.stderr)
                self.assertEqual(self.calls(), [])

    def test_empty_selection_fails(self):
        manifest = self.directory / "empty.csv"
        manifest.write_text("# no destinations\n")
        result = self.run_publish(manifest=manifest)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("No repositories selected", result.stderr)
        self.assertEqual(self.calls(), [])

    def test_invalid_scope_fails(self):
        self.assertNotEqual(self.run_publish("azurelinux3").returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_invalid_repository_id_fails_before_any_upload(self):
        self.azure_artifacts()
        manifest = self.manifest_with_repo("AzureLinux-4.0-x86_64", "--unexpected-option")
        self.assertNotEqual(self.run_publish(manifest=manifest).returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_unknown_package_type_fails_before_any_upload(self):
        self.azure_artifacts()
        manifest = self.directory / "invalid.csv"
        manifest.write_text(
            f"AzureLinux-4.0-x86_64,unknown,{AZURE_LINUX_4_REPOS[0]},\n"
        )
        result = self.run_publish(manifest=manifest)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Unsupported package type", result.stderr)
        self.assertEqual(self.calls(), [])


if __name__ == "__main__":
    unittest.main()
