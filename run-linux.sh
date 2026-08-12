#!/usr/bin/env bash
set -euo pipefail

# Bash run script for M.A.X. GO x86-64.
# Suglasp

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$script_dir/build/linux-x64"
bin_dir="$script_dir/bin"

#if [[ -t 1 && -n "${TERM:-}" ]] && command -v clear >/dev/null 2>&1; then
#  clear || true
#fi

echo
echo "Running M.A.X. ..."

if [[ ! -f "$bin_dir/max-go" ]]; then
  echo "Err: No M.A.X. binary?"
  read -rn1 -p "Press any key to continue..." || true
  echo
  exit 1
fi

if [[ ! -f "$bin_dir/max.res" ]]; then
  echo "Err: No M.A.X. data in folder?"
  read -rn1 -p "Press any key to continue..." || true
  echo
  exit 1
fi

"$bin_dir/max-go"

echo
echo "END RUN."
