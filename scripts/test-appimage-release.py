import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("check_appdir", SCRIPTS / "check-appdir.py")
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class AppDirTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name) / "AppDir"
        self.root.mkdir(mode=0o755)
        for name in ["AppRun", "usr/bin/rainforest", "rainforest.desktop", "rainforest.png",
                     "usr/lib/libgtk-3.so.0", "usr/lib/libepoxy.so.0", "usr/lib/libxkbcommon-x11.so.0",
                     "usr/lib/libxcb-xkb.so.1", "usr/lib/libwebkit2gtk-4.1.so.0",
                     "usr/lib/libjavascriptcoregtk-4.1.so.0", "usr/lib/libsoup-3.0.so.0",
                     "usr/lib/x86_64-linux-gnu/webkit2gtk-4.1/WebKitNetworkProcess",
                     "usr/lib/x86_64-linux-gnu/webkit2gtk-4.1/WebKitWebProcess"]:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("test fixture")
            path.chmod(0o755 if name in ["AppRun", "usr/bin/rainforest"] or name.endswith("Process") else 0o644)
        for path in self.root.rglob("*"):
            if path.is_dir():
                path.chmod(0o755)

    def test_complete_world_readable_appdir_passes(self):
        self.assertEqual(checker.check_appdir(self.root), [])

    def test_owner_only_launcher_is_rejected(self):
        (self.root / "AppRun").chmod(0o744)
        self.assertTrue(any("not executable" in error for error in checker.check_appdir(self.root)))

    def test_owner_only_parent_directory_is_rejected(self):
        (self.root / "usr").chmod(0o700)
        self.assertTrue(any("not readable/traversable" in error for error in checker.check_appdir(self.root)))

    def test_owner_only_resource_is_rejected(self):
        (self.root / "rainforest.png").chmod(0o600)
        self.assertTrue(any("not readable/traversable" in error for error in checker.check_appdir(self.root)))

    def test_missing_dlopen_dependency_is_rejected(self):
        (self.root / "usr/lib/libxkbcommon-x11.so.0").unlink()
        self.assertIn("missing dynamically loaded library: libxkbcommon-x11.so.0", checker.check_appdir(self.root))

    def test_nonexecutable_webkit_helper_is_rejected(self):
        (self.root / "usr/lib/x86_64-linux-gnu/webkit2gtk-4.1/WebKitWebProcess").chmod(0o644)
        self.assertIn("missing executable WebKit helper: WebKitWebProcess", checker.check_appdir(self.root))

    def test_host_dependency_symlink_is_rejected(self):
        path = self.root / "usr/lib/libgtk-3.so.0"
        path.unlink()
        path.symlink_to("/etc/hosts")
        self.assertTrue(any("symlink escapes" in error for error in checker.check_appdir(self.root)))

    def test_internal_library_symlink_is_accepted(self):
        path = self.root / "usr/lib/libgtk-3.so.0"
        path.rename(path.with_name("libgtk-real.so"))
        path.symlink_to("libgtk-real.so")
        self.assertEqual(checker.check_appdir(self.root), [])

    def test_broken_symlink_is_rejected(self):
        (self.root / "broken").symlink_to("missing")
        self.assertTrue(any("unreadable path" in error for error in checker.check_appdir(self.root)))


class LauncherTests(unittest.TestCase):
    def test_launcher_uses_bundled_binary_hooks_and_preserves_arguments(self):
        with tempfile.TemporaryDirectory(prefix="app image ") as directory:
            root = Path(directory)
            (root / "usr/bin").mkdir(parents=True)
            (root / "apprun-hooks").mkdir()
            (root / "apprun-hooks/test.sh").write_text("export PACKAGED_HOOK=present\n")
            binary = root / "usr/bin/rainforest"
            binary.write_text('#!/bin/sh\nprintf "%s\\n" "$PACKAGED_HOOK" "$APPDIR" "$LD_LIBRARY_PATH" "$@"\n')
            binary.chmod(0o755)
            env = dict(os.environ, APPDIR=str(root))
            result = subprocess.run(["sh", str(SCRIPTS.parent / "packaging/appimage/AppRun"), "a b", "--version"],
                                    env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            lines = result.stdout.splitlines()
            self.assertEqual(lines[:2], ["present", str(root)])
            self.assertTrue(lines[2].startswith(f"{root}/usr/lib:"))
            self.assertEqual(lines[3:], ["a b", "--version"])


if __name__ == "__main__":
    unittest.main()
