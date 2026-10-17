#!/usr/bin/env bash
set -euo pipefail
[[ $# == 1 ]] || { echo "usage: $0 TOOL_DIRECTORY" >&2; exit 2; }
mkdir -p "$1"
tools=$(realpath "$1")
sudo apt-get update
sudo apt-get install -y --no-install-recommends libgtk-3-dev libgirepository1.0-dev librsvg2-dev \
  libwebkit2gtk-4.1-dev libayatana-appindicator3-dev libsecret-1-dev \
  libxkbcommon-x11-dev libxcb-xkb-dev patchelf desktop-file-utils zsync file
curl --fail --location --retry 3 https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage -o "$tools/linuxdeploy"
printf '%s  %s\n' 8aea8da0f7f7039d2a2cecb14657d752a222a5e1d3825caeef186c82f751cdd1 "$tools/linuxdeploy" | sha256sum -c -
curl --fail --location --retry 3 https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/7a3fbc31a9e5075073ff8790f26effbac5f84453/linuxdeploy-plugin-gtk.sh -o "$tools/linuxdeploy-plugin-gtk.sh"
chmod 755 "$tools/linuxdeploy" "$tools/linuxdeploy-plugin-gtk.sh"
