#!/usr/bin/env bash
set -euo pipefail
if [[ $# != 2 ]]; then echo "usage: $0 MYGO_LINUX_AMD64_DIR OUTPUT_DIR" >&2; exit 2; fi
bundle=$(realpath "$1")
mkdir -p "$2"
output_dir=$(realpath "$2")
root=$(cd "$(dirname "$0")/.." && pwd)
version=$(python3 -c 'import re; from pathlib import Path; print(re.search(r"^\s*version:\s*\"([^\"]+)\"", Path("mygo.config.ts").read_text(), re.M)[1])')
[[ $(uname -m) == x86_64 ]] || { echo 'AppImage packaging requires x86_64 Linux' >&2; exit 1; }
for file in rainforest rainforest.desktop rainforest.png; do test -s "$bundle/$file"; done
test -x "$bundle/rainforest"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
appdir="$work/AppDir"
mkdir -p "$appdir/usr/bin" "$appdir/usr/lib/x86_64-linux-gnu" "$appdir/usr/share/applications" "$appdir/usr/share/icons/hicolor/256x256/apps"
# MyGo puts application resources next to its executable; exclude only release containers/metadata.
for source in "$bundle"/*; do
  case "${source##*/}" in
    *.deb|*.tar.gz|*.AppImage|*.zsync|update-*.json|install.sh|rainforest.desktop|rainforest.png) continue ;;
  esac
  cp -a "$source" "$appdir/usr/bin/"
done
install -m 644 "$bundle/rainforest.desktop" "$appdir/usr/share/applications/rainforest.desktop"
install -m 644 "$bundle/rainforest.png" "$appdir/usr/share/icons/hicolor/256x256/apps/rainforest.png"
install -m 755 "$root/packaging/appimage/AppRun" "$appdir/AppRun"
# MyGo uses purego Dlopen: these libraries are invisible to ELF DT_NEEDED traversal.
libraries=(libgtk-3.so.0 libgdk-3.so.0 libglib-2.0.so.0 libgobject-2.0.so.0 libgio-2.0.so.0
  libcairo.so.2 libgdk_pixbuf-2.0.so.0 libpango-1.0.so.0 libpangocairo-1.0.so.0
  libpangoft2-1.0.so.0 libharfbuzz.so.0 libfontconfig.so.1 libatk-1.0.so.0 libepoxy.so.0
  libX11.so.6 libxkbcommon-x11.so.0 libxcb-xkb.so.1 libsecret-1.so.0 libfribidi.so.0
  libthai.so.0 libdatrie.so.1 libfreetype.so.6 libpng16.so.16 libexpat.so.1 libbrotlidec.so.1 libbrotlicommon.so.1
  libayatana-appindicator3.so.1 libwebkit2gtk-4.1.so.0 libjavascriptcoregtk-4.1.so.0 libsoup-3.0.so.0)
args=()
for library in "${libraries[@]}"; do
  path="/usr/lib/x86_64-linux-gnu/$library"
  test -s "$path"
  args+=(--library "$path")
done
# WebKit launches helpers by path, not through DT_NEEDED.
webkit=/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1
test -d "$webkit"
cp -a "$webkit" "$appdir/usr/lib/x86_64-linux-gnu/"
for helper in "$webkit"/WebKit*Process; do
  test -x "$helper"
  args+=(--library "$helper")
done
export ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 DEPLOY_GTK_VERSION=3
export LDAI_UPDATE_INFORMATION='gh-releases-zsync|thedavidweng|rainforest|latest|rainforest-*-x86_64.AppImage.zsync'
export LDAI_OUTPUT="$output_dir/rainforest-${version}-x86_64.AppImage"
# Keep appimagetool's relative .zsync output out of the repository.
cd "$work"
linuxdeploy --appdir "$appdir" --plugin gtk "${args[@]}"
chmod -R a+rX "$appdir"
chmod a+rx "$appdir/AppRun" "$appdir/usr/bin/rainforest"
python3 "$root/scripts/check-appdir.py" "$appdir"
desktop-file-validate "$appdir/rainforest.desktop"
linuxdeploy --appdir "$appdir" --output appimage
if [[ -f "${LDAI_OUTPUT##*/}.zsync" ]]; then mv "${LDAI_OUTPUT##*/}.zsync" "${LDAI_OUTPUT}.zsync"; fi
test -s "$LDAI_OUTPUT"
test -s "${LDAI_OUTPUT}.zsync"
chmod a+rx "$LDAI_OUTPUT"
