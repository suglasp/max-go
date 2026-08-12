#!/usr/bin/env bash
set -euo pipefail

# Bash run script for M.A.X. GO x86-64.
# Suglasp

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Stage the Linux build into bin/, which is the folder the game runs from.
build_dir="$script_dir/build/linux-x64"
bin_dir="$script_dir/bin"

#if [[ -t 1 && -n "${TERM:-}" ]] && command -v clear >/dev/null 2>&1; then
#  clear || true
#fi

mkdir -p "$bin_dir"

if [[ -d "$build_dir" ]]; then
  echo
  echo "Prepping M.A.X. files ..."
  # Copy binaries and help files; ignore missing silently. Each binary is named in
  # full, never swept up by a max-go* / max-* glob: that would also match the
  # max-go.json template and overwrite the live settings the player has in bin/.
  for tool in max-go max-restool max-convert max-settings; do
    cp -f "$build_dir/$tool" "$bin_dir/" 2>/dev/null || true
  done
  cp -f "$build_dir"/max-*-help.txt "$bin_dir/" 2>/dev/null || true

  # Copy any SoLoud / miniaudio shared libraries present in the build folder.
  cp -f "$build_dir"/libsoloud* "$bin_dir/" 2>/dev/null || true
  cp -f "$build_dir"/libminiaudio* "$bin_dir/" 2>/dev/null || true
  cp -f "$build_dir/README.md" "$bin_dir/" 2>/dev/null || true
  cp -f "$build_dir/max-lavender.png" "$bin_dir/" 2>/dev/null || true
  
  # The port's own UI string file; the game expects it beside the executable.
  cp -f "$build_dir/lang-en-us.ini" "$bin_dir/" 2>/dev/null || true

  # The release version stamp, refreshed every stage so bin/ never reports a stale build.
  cp -f "$build_dir/version" "$bin_dir/" 2>/dev/null || true

  # Attribution for the bundled audio libraries: it must sit beside the binaries we ship.
  cp -f "$build_dir"/*-LICENSE* "$bin_dir/" 2>/dev/null || true
  cp -f "$build_dir"/*-readme* "$bin_dir/" 2>/dev/null || true

  # Seed the settings template only into a fresh bin.
  # the player's live settings are bin/max-go.json (the game runs from bin/ and writes them there),
  # so a stage must never overwrite it. The legacy settings.json is still READ if a player has one
  # (max-go.json wins), but it is no longer shipped, so nothing seeds it here.
  if [[ ! -f "$bin_dir/max-go.json" ]]; then
    cp -f "$build_dir/max-go.json" "$bin_dir/max-go.json" 2>/dev/null || true
  fi
fi

echo
echo "---------------------------------------"
echo "Build done. Run run-linux.sh to launch."
echo
