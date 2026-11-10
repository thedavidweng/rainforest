#!/usr/bin/env bash
set -euo pipefail
[[ $# == 1 ]] || { echo "usage: $0 TOOL_DIRECTORY" >&2; exit 2; }
mkdir -p "$1"
tools=$(realpath "$1")
sudo apt-get update
sudo apt-get install -y --no-install-recommends libgtk-3-dev libgirepository1.0-dev librsvg2-dev \
  libwebkit2gtk-4.1-dev libayatana-appindicator3-dev libsecret-1-dev \
  libxkbcommon-x11-dev libxcb-xkb-dev patchelf desktop-file-utils zsync file
curl --fail --location --retry 3 https://github.com/linuxdeploy/linuxdeploy/releases/download/1-alpha-20251107-1/linuxdeploy-x86_64.AppImage -o "$tools/linuxdeploy"
printf '%s  %s\n' c20cd71e3a4e3b80c3483cef793cda3f4e990aca14014d23c544ca3ce1270b4d "$tools/linuxdeploy" | sha256sum -c -
curl --fail --location --retry 3 https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage -o "$tools/appimagetool"
printf '%s  %s\n' ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0 "$tools/appimagetool" | sha256sum -c -
curl --fail --location --retry 3 https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/7a3fbc31a9e5075073ff8790f26effbac5f84453/linuxdeploy-plugin-gtk.sh -o "$tools/linuxdeploy-plugin-gtk.sh"
chmod 755 "$tools/linuxdeploy" "$tools/appimagetool" "$tools/linuxdeploy-plugin-gtk.sh"
