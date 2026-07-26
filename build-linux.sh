#!/usr/bin/env bash
set -euo pipefail

# Suglasp
# Bash build script for M.A.X. GO x86-64.

export GOARCH=amd64

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$script_dir/build"

os_name="$(uname -s)"

# function to track dependencies on Linux
check_linux_dependencies() {
  if [[ "$os_name" != "Linux" ]]; then
    return
  fi

  local missing=()
  for cmd in go gcc g++ pkg-config; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      missing+=("$cmd")
    fi
  done

  if command -v pkg-config >/dev/null 2>&1; then
    local pkg
    for pkg in alsa gl x11 xcursor xrandr xinerama xi xxf86vm; do
      if ! pkg-config --exists "$pkg"; then
        missing+=("$pkg.pc")
      fi
    done
  fi

  if ((${#missing[@]} == 0)); then
    return
  fi

  echo "Missing Linux build dependencies:"
  printf '  %s\n' "${missing[@]}"
  echo
  echo "Fedora/Alma/RHEL:"
  echo "  sudo dnf install gcc gcc-c++ pkgconf-pkg-config alsa-lib-devel libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel libglvnd-devel mesa-libGL-devel"
  echo
  echo "Debian/Ubuntu:"
  echo "  sudo apt install build-essential pkg-config libasound2-dev libgl1-mesa-dev libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libxxf86vm-dev"
  exit 1
}

write_help() {
  local output="$1"
  shift
  local rc=0

  "$@" > "$output" 2>&1 || rc=$?
  if ((rc != 0 && rc != 2)); then
    return "$rc"
  fi
}

write_max_go_help() {
  local output="$build_dir/max-go-help.txt"

  if [[ "$os_name" == "Linux" && -z "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
    cat > "$output" <<'EOF'
max-go help was not generated on this build host.

The current max-go buildary imports Ebitengine before Go flag parsing, and
Ebitengine requires DISPLAY or WAYLAND_DISPLAY during process startup on Linux.
Run `max-go -help` on Windows or on a Linux desktop session to print full CLI help.
EOF
    return 0
  fi

  write_help "$output" "$build_dir/max-go$exe" -h
}

# build
case "$os_name" in
  MINGW*|MSYS*|CYGWIN*)
    exe=".exe"
    ;;
  *)
    exe=""
    ;;
esac

if [[ -t 1 && -n "${TERM:-}" ]] && command -v clear >/dev/null 2>&1; then
  clear || true
fi

echo
echo "Prepping ..."
check_linux_dependencies
go mod tidy

mkdir -p "$build_dir"

echo
echo "Building M.A.X. in Golang ..."
go build -o "$build_dir/" ./cmd/max-go

echo
echo "Building M.A.X. utilities ..."
go build -o "$build_dir/" ./cmd/max-restool
go build -o "$build_dir/" ./cmd/max-convert
go build -o "$build_dir/" ./cmd/max-settings

echo
echo "Writing CLI help ..."
write_max_go_help
write_help "$build_dir/max-restool-help.txt" "$build_dir/max-restool$exe" help
write_help "$build_dir/max-convert-help.txt" "$build_dir/max-convert$exe" help
write_help "$build_dir/max-settings-help.txt" "$build_dir/max-settings$exe" -h

echo
echo "Copying documentation ..."
cp -f "$script_dir/README.md" "$build_dir/"

echo
echo "Copying configuration files ..."
# The port's own UI string file; the game expects it beside the executable.
cp -f "$script_dir/lang-en-us.ini" "$build_dir/"

# The settings template. build/ is pure output, so it is refreshed every build:
# the player's live settings live in bin/max-go.json and are never touched from here.
# The (beta) legacy settings.json is still read if a player has one, but it is no longer shipped.
cp -f "$script_dir/max-go.json" "$build_dir/"

# The release version stamp. It ships beside the binary so a player can tell which build they have without launching the game.
cp -f "$script_dir/version" "$build_dir/"


# Copy any SoLoud / miniaudio shared libraries into the build folder.
shopt -s nullglob
soloud_files=("$script_dir"/dependencies/soloud-build/*.so "$script_dir"/dependencies/soloud-bin/*)
if ((${#soloud_files[@]} > 0)); then
  cp -f "${soloud_files[@]}" "$build_dir/"
fi

miniaudio_files=("$script_dir"/dependencies/miniaudio-bin/*.so "$script_dir"/dependencies/miniaudio-bin/*-LICENSE*)
if ((${#miniaudio_files[@]} > 0)); then
  cp -f "${miniaudio_files[@]}" "$build_dir/"
fi

echo
echo "---------------------------------------"
echo "Build done. Run stage-linux.sh to stage."
echo

