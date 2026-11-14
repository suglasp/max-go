#!/usr/bin/env bash
set -euo pipefail

# Bash run script for M.A.X. GO x86-64. in a random game
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

# Old check, it only accepted the lower case name and ignored a configured data folder:
# if [[ ! -f "$bin_dir/max.res" ]]; then
#   echo "Err: No M.A.X. data in folder?"
#   read -rn1 -p "Press any key to continue..." || true
#   echo
#   exit 1
# fi
# A data folder named by -datadir or by data_dir in max-go.json is checked by the game itself,
# so the script only checks its own folder when neither is given. The json test is a plain text
# grep for a non empty "data_dir" value, not a real parse.
has_datadir=0
for arg in "$@"; do
  case "$arg" in
    -datadir|--datadir|-datadir=*|--datadir=*) has_datadir=1 ;;
  esac
done
if [[ -f "$bin_dir/max-go.json" ]] && grep -Eq '"data_dir"[[:space:]]*:[[:space:]]*"[^"]' "$bin_dir/max-go.json"; then
  has_datadir=1
fi

# The retail data file is MAX.RES, but accept any letter case: Linux file names are case sensitive.
has_data=0
for f in "$bin_dir"/*; do
  if [[ -f "$f" && "${f##*/}" =~ ^[Mm][Aa][Xx]\.[Rr][Ee][Ss]$ ]]; then
    has_data=1
    break
  fi
done

if [[ "$has_datadir" != "1" && "$has_data" != "1" ]]; then
  echo "Err: No M.A.X. data in folder?"
  read -rn1 -p "Press any key to continue..." || true
  echo
  exit 1
fi

# Old launch, extra arguments were not passed on:
# "$bin_dir/max-go" -random -difficulty 1 -npc 1 -derelicts common
"$bin_dir/max-go" -random -difficulty 1 -npc 1 -derelicts common "$@"

echo
echo "END RUN."
