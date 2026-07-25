#!/usr/bin/env bash
set -euo pipefail

# Suglasp
# Regenerate the Windows icon resource files for cmd/max-go.
# Override these variables locally if your tool or icon layout differs.

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package_dir="$script_dir/cmd/max-go"

WINDRES="${WINDRES:-windres}"
WINDRES_TARGET="${WINDRES_TARGET:-pe-x86-64}"
ICON_DIR="${ICON_DIR:-$script_dir/ico}"
ICON_NAME="${ICON_NAME:-max-go.ico}"
RC_DIR="${RC_DIR:-$package_dir}"
RC_NAME="${RC_NAME:-max_icon.rc}"
SYSO_NAME="${SYSO_NAME:-max_icon_windows_amd64.syso}"
RC_ONLY="${RC_ONLY:-0}"

icon_path="$ICON_DIR/$ICON_NAME"
rc_path="$RC_DIR/$RC_NAME"
syso_path="$RC_DIR/$SYSO_NAME"

if [[ -z "${ICON_REF:-}" ]]; then
  if [[ "$(cd "$ICON_DIR" && pwd)" == "$(cd "$script_dir/ico" && pwd)" ]]; then
    ICON_REF="../../ico/$ICON_NAME"
  elif [[ "$(cd "$ICON_DIR" && pwd)" == "$(cd "$RC_DIR" && pwd)" ]]; then
    ICON_REF="$ICON_NAME"
  else
    ICON_REF="$icon_path"
  fi
fi

if [[ ! -f "$icon_path" ]]; then
  echo "Icon not found: $icon_path" >&2
  exit 1
fi

mkdir -p "$RC_DIR"
printf 'IDI_ICON1 ICON "%s"\n' "$ICON_REF" > "$rc_path"
echo "Wrote $rc_path"

if [[ "$RC_ONLY" == "1" ]]; then
  exit 0
fi

if [[ ! -x "$WINDRES" ]] && ! command -v "$WINDRES" >/dev/null 2>&1; then
  echo "windres tool not found: $WINDRES" >&2
  echo "Set WINDRES to your windres path, for example /usr/bin/x86_64-w64-mingw32-windres." >&2
  exit 1
fi

windres_cmd="$WINDRES"
if [[ -f "$WINDRES" ]]; then
  windres_cmd="$(cd "$(dirname "$WINDRES")" && pwd)/$(basename "$WINDRES")"
fi

(
  cd "$RC_DIR"
  "$windres_cmd" --target="$WINDRES_TARGET" -i "$RC_NAME" -o "$SYSO_NAME"
)
echo "Wrote $syso_path"
