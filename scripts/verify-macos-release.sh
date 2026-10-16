#!/usr/bin/env bash
set -euo pipefail

: "${APPLE_TEAM_ID:?}"
stage="${1:?Usage: verify-macos-release.sh STAGE}"
artifacts=("$stage/Rainforest.app" "$stage"/*.dmg)
test "${#artifacts[@]}" -eq 2
for artifact in "${artifacts[@]}"; do
  test -e "$artifact"
  codesign --verify --deep --strict --verbose=2 "$artifact"
  details="$(codesign --display --verbose=4 "$artifact" 2>&1)"
  grep -Fx "TeamIdentifier=$APPLE_TEAM_ID" <<< "$details"
  grep -F 'Authority=Developer ID Application:' <<< "$details"
  xcrun stapler validate "$artifact"
  if [[ "$artifact" == *.app ]]; then
    spctl --assess --type execute --verbose=2 "$artifact"
  else
    spctl --assess --type open --context context:primary-signature --verbose=2 "$artifact"
  fi
done
