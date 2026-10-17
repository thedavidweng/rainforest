#!/usr/bin/env bash
set -euo pipefail
[[ $# == 1 ]] || { echo "usage: $0 APPIMAGE" >&2; exit 2; }
[[ $(id -u) != 0 ]] || { echo 'GUI smoke must not run as root' >&2; exit 1; }
# --network none must be supplied by the caller; assert no non-loopback interface.
for interface in /sys/class/net/*; do
  [[ ${interface##*/} == lo ]] || { echo 'GUI smoke must be offline' >&2; exit 1; }
done
image=$(realpath "$1")
[[ -x "$image" && -s "${image}.zsync" ]]
expected='gh-releases-zsync|thedavidweng|rainforest|latest|rainforest-*-x86_64.AppImage.zsync'
actual=$(env -u APPIMAGE_EXTRACT_AND_RUN "$image" --appimage-updateinformation 2>&1)
[[ "$actual" == "$expected" ]] || { printf 'unexpected update information: %s\n' "$actual" >&2; exit 1; }
grep -Fx "Filename: ${image##*/}" "${image}.zsync"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"
env -u APPIMAGE_EXTRACT_AND_RUN "$image" --appimage-extract > /dev/null
python3 /checks/check-appdir.py "$work/squashfs-root"
export HOME="$work/home" XDG_CONFIG_HOME="$work/home/.config" XDG_DATA_HOME="$work/home/.local/share"
export XDG_CACHE_HOME="$work/home/.cache" XDG_RUNTIME_DIR="$work/runtime"
mkdir -m 700 -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_CACHE_HOME" "$XDG_RUNTIME_DIR"
export LANG=C.UTF-8 GDK_BACKEND=x11 LIBGL_ALWAYS_SOFTWARE=1 APPIMAGE_EXTRACT_AND_RUN=1
export IMAGE="$image" SMOKE_WORK="$work"
dbus-run-session -- xvfb-run -a -s '-screen 0 1280x900x24 -nolisten tcp' bash <<'GUI'
set -euo pipefail
"$IMAGE" > "$SMOKE_WORK/gui.log" 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; cat "$SMOKE_WORK/gui.log"' EXIT
window=''
for attempt in {1..60}; do
  kill -0 "$pid" || { echo 'AppImage exited before mapping its window' >&2; exit 1; }
  window=$(xdotool search --onlyvisible --name '^Rainforest$' 2>/dev/null | sort -n | uniq | tr '\n' ' ' || true)
  if [[ -n "$window" ]]; then break; fi
  sleep 1
done
[[ -n "$window" ]] || { echo 'No visible Rainforest window' >&2; exit 1; }
read -r -a windows <<< "$window"
[[ ${#windows[@]} == 1 ]] || { echo 'Expected exactly one Rainforest main window' >&2; exit 1; }
owner=$(xdotool getwindowpid "${windows[0]}")
[[ $(readlink "/proc/$owner/exe") == */rainforest ]]
[[ $(stat -c %u "/proc/$owner") == "$(id -u)" ]]
geometry=$(xdotool getwindowgeometry --shell "${windows[0]}")
printf '%s\n' "$geometry"
width=$(printf '%s\n' "$geometry" | grep '^WIDTH=' | cut -d= -f2)
height=$(printf '%s\n' "$geometry" | grep '^HEIGHT=' | cut -d= -f2)
[[ "$width" -ge 860 && "$height" -ge 580 ]]
sleep 5
kill -0 "$pid"
xdotool search --onlyvisible --all --pid "$owner" --name '^Rainforest$' > /dev/null
# Initialization must reach SQLite, not merely display an error dialog.
find "$HOME" -name rainforest.db -type f -size +0c | grep -q .
GUI
