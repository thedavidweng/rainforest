#!/usr/bin/env python3
from pathlib import Path
import stat
import sys


def check_appdir(root):
    root = Path(root).resolve()
    errors = []
    for path in [root, *root.rglob("*")]:
        target = path.resolve()
        if not target.is_relative_to(root):
            errors.append(f"symlink escapes AppDir: {path}")
            continue
        try:
            mode = target.stat().st_mode
        except OSError as error:
            errors.append(f"unreadable path {path}: {error}")
            continue
        required = 0o555 if stat.S_ISDIR(mode) else 0o444
        if mode & required != required:
            errors.append(f"not readable/traversable by non-owner: {path}")
    for name in ["AppRun", "usr/bin/rainforest"]:
        path = root / name
        if not path.is_file() or path.stat().st_mode & 0o555 != 0o555:
            errors.append(f"not executable by non-owner: {path}")
    for name in ["libgtk-3.so.0", "libepoxy.so.0", "libxkbcommon-x11.so.0", "libxcb-xkb.so.1",
                 "libwebkit2gtk-4.1.so.0", "libjavascriptcoregtk-4.1.so.0", "libsoup-3.0.so.0"]:
        if not any(path.is_file() for path in (root / "usr/lib").rglob(name)):
            errors.append(f"missing dynamically loaded library: {name}")
    for name in ["WebKitNetworkProcess", "WebKitWebProcess"]:
        path = root / "usr/lib/x86_64-linux-gnu/webkit2gtk-4.1" / name
        if not path.is_file() or path.stat().st_mode & 0o555 != 0o555:
            errors.append(f"missing executable WebKit helper: {name}")
    for name in ["rainforest.desktop", "rainforest.png"]:
        if not (root / name).is_file():
            errors.append(f"missing desktop integration: {name}")
    return errors


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: check-appdir.py APPDIR")
    errors = check_appdir(sys.argv[1])
    if errors:
        sys.exit("\n".join(errors))
