#!/usr/bin/env python3
"""Exercise source installation only inside temporary roots."""

import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class Installation(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        subprocess.run(["make", "build"], cwd=ROOT, check=True)
        cls.binary = ROOT / "_workspace/bin/iris"

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="iris-install-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.prefix = self.root / "prefix with % and $ signs"
        self.units = self.root / "units"
        self.env = dict(os.environ, XDG_DATA_HOME=str(self.root / "data"),
                        XDG_CONFIG_HOME=str(self.root / "config"),
                        XDG_RUNTIME_DIR=str(self.root / "runtime with % and $ signs"))
        self.command = ["python3", str(ROOT / "scripts/install.py")]
        self.paths = ["--prefix", str(self.prefix), "--unit-dir", str(self.units)]

    def run_install(self, action="install", success=True):
        result = subprocess.run(self.command + [action] + self.paths, env=self.env,
                                text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode == 0, success, result.stderr)
        return result

    # Repeated builds omit mutable source paths and produce the same executable.
    def test_reproducible_build(self):
        before = hashlib.sha256(self.binary.read_bytes()).hexdigest()
        subprocess.run(["make", "build"], cwd=ROOT, check=True)
        self.assertEqual(hashlib.sha256(self.binary.read_bytes()).hexdigest(), before)

    # Installation and upgrade are atomic file operations with explicit activation.
    def test_install_upgrade_remove(self):
        self.run_install()
        binary = self.prefix / "bin/iris"
        self.assertEqual(binary.read_bytes(), self.binary.read_bytes())
        result = subprocess.run([str(binary), "--help"], env=self.env,
                                text=True, capture_output=True, check=True)
        self.assertIn("iris <command>", result.stdout)
        self.assertEqual(binary.stat().st_mode & 0o777, 0o755)
        self.assertEqual((self.units / "iris.service").stat().st_mode & 0o777, 0o600)
        service = (self.units / "iris.service").read_text()
        self.assertIn("prefix with %% and $ signs/bin/iris", service)
        self.assertIn("XDG_DATA_HOME=" + self.env["XDG_DATA_HOME"], service)
        socket = (self.units / "iris.socket").read_text()
        self.assertIn(self.env["XDG_RUNTIME_DIR"].replace("%", "%%") + "/iris/iris.sock", socket)
        subprocess.run(["systemd-analyze", "--user", "verify", str(self.units / "iris.service"),
                        str(self.units / "iris.socket")], check=True)
        inode = binary.stat().st_ino
        self.run_install()
        self.assertNotEqual(binary.stat().st_ino, inode)
        for root, name in ((self.root / "data/iris", "iris.db"),
                           (self.root / "config/iris", "config.json")):
            root.mkdir(parents=True)
            (root / name).write_text("preserve")
        self.run_install("uninstall")
        self.assertFalse(binary.exists())
        self.assertFalse((self.units / "iris.service").exists())
        self.assertFalse((self.units / "iris.socket").exists())
        self.assertEqual((self.root / "data/iris/iris.db").read_text(), "preserve")
        self.assertEqual((self.root / "config/iris/config.json").read_text(), "preserve")

    # A symlink destination must not overwrite or remove another file.
    def test_unsafe_destination(self):
        self.units.mkdir()
        preserved = self.root / "preserved"
        preserved.write_text("preserve")
        (self.units / "iris.service").symlink_to(preserved)
        self.run_install(success=False)
        self.run_install("uninstall", success=False)
        self.assertEqual(preserved.read_text(), "preserve")
        self.assertFalse((self.prefix / "bin/iris").exists())


if __name__ == "__main__":
    unittest.main()
