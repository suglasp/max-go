#!/usr/bin/env bash
set -euo pipefail

# Suglasp
# Bash build script for M.A.X. GO x86-64.

export GOARCH=amd64

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

os_name="$(uname -s)"

# Keep each platform in its own build folder so Windows and Linux outputs coexist.
case "$os_name" in
  MINGW*|MSYS*|CYGWIN*)
    exe=".exe"
    platform="windows-x64"
    ;;
  *)
    exe=""
    platform="linux-x64"
    ;;
esac

build_dir="$script_dir/build/$platform"
win_dir="$script_dir/build/windows-x64"
pack_dir="$script_dir/build/pack"
release_dir="$script_dir/release"

# The first version-file line names the release archives.
version="$(head -n 1 "$script_dir/version" | tr -d '\r')"

# Let unmatched globs expand to nothing so optional payload files need no special case.
shopt -s nullglob

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

# Copy matched files only.
copy_if_any() {
  local dest="$1"
  shift

  if (($# > 0)); then
    cp -f "$@" "$dest/"
  fi
}

# Copy the common release payload.
copy_payload() {
  local dest="$1"

  cp -f "$script_dir/README.md" "$dest/"
  cp -f "$script_dir/max-lavender.png" "$dest/"
  # LANG.RES (UI strings + unit descriptions) is packed once into the build folder, beside the executable.
  if [[ "$dest" != "$build_dir" ]]; then
    cp -f "$build_dir/LANG.RES" "$dest/"
  fi
  # Refresh the build-only settings template. Live settings remain in bin/max-go.json.
  # The game still reads a legacy settings.json when present, but releases do not ship one.
  cp -f "$script_dir/max-go.json" "$dest/"
  # Ship the version stamp beside the executable.
  cp -f "$script_dir/version" "$dest/"
  # Ship license and readme files for the bundled audio libraries.
  copy_if_any "$dest" "$script_dir"/dependencies/soloud-bin/*-LICENSE* "$script_dir"/dependencies/soloud-bin/*-readme*
  copy_if_any "$dest" "$script_dir"/dependencies/miniaudio-bin/*-LICENSE*
}

# Copy the matching native audio libraries so each archive contains only its platform's files.
copy_native_libs() {
  local dest="$1"
  local plat="$2"

  if [[ "$plat" == "windows-x64" ]]; then
    copy_if_any "$dest" "$script_dir"/dependencies/soloud-bin/*.dll "$script_dir"/dependencies/miniaudio-bin/*.dll
    return
  fi

  copy_if_any "$dest" "$script_dir"/dependencies/soloud-build/*_x64.so "$script_dir"/dependencies/soloud-bin/*_x64.so "$script_dir"/dependencies/miniaudio-bin/*_x64.so
}

# Pack a platform folder only when it contains the game binary.
pack_platform() {
  local plat="$1"
  local dir="$script_dir/build/$plat"
  local name="max-go-$version-$plat"
  local game="$dir/max-go"

  if [[ "$plat" == "windows-x64" ]]; then
    game="$dir/max-go.exe"
  fi

  if [[ ! -f "$game" ]]; then
    echo "  skipped $plat, $(basename "$game") is not there"
    return
  fi

  mkdir -p "$pack_dir" "$release_dir"
  rm -rf "${pack_dir:?}/$name"
  # Copy under the release name so the archive unpacks into one folder.
  cp -a "$dir" "$pack_dir/$name"

  # Replace an archive only after confirming the required packer is available.
  if [[ "$plat" == "windows-x64" ]]; then
    if command -v zip >/dev/null 2>&1; then
      rm -f "$release_dir/$name.zip"
      ( cd "$pack_dir" && zip -qr "$release_dir/$name.zip" "$name" )
      echo "  release/$name.zip"
    else
      echo "  skipped $name.zip, zip is not installed"
    fi
  else
    if command -v zstd >/dev/null 2>&1; then
      rm -f "$release_dir/$name.tar.zst"
      tar --zstd -cf "$release_dir/$name.tar.zst" -C "$pack_dir" "$name"
      echo "  release/$name.tar.zst"
    else
      echo "  skipped $name.tar.zst, zstd is not installed"
    fi
  fi

  rm -rf "${pack_dir:?}/$name"
}

if [[ -t 1 && -n "${TERM:-}" ]] && command -v clear >/dev/null 2>&1; then
  clear || true
fi

echo
echo "Prepping ..."
check_linux_dependencies
go mod tidy

# Rebuild the native output folder so stale files cannot enter a release archive.
rm -rf "${build_dir:?}"
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
echo "Packing LANG.RES ..."
# The port's addon archive: language strings + unit description keys, packed from langres/.
"$build_dir/max-restool$exe" pack "$script_dir/langres" "$build_dir/LANG.RES"

echo
echo "Copying documentation and configuration files ..."
copy_payload "$build_dir"
copy_native_libs "$build_dir" "$platform"

# Linux can cross-build the complete Windows release because Ebitengine's Windows build is pure Go.
# Ebitengine 2.10 also drives X11 and OpenGL from pure Go, so Windows cross-builds the Linux game too.
if [[ "$platform" == "linux-x64" ]]; then
  echo
  echo "Cross building M.A.X. for Windows x64 ..."
  mkdir -p "$win_dir"
  # Keep Windows-generated help text while removing Linux shared objects.
  rm -f "$win_dir"/*.so
  GOOS=windows CGO_ENABLED=0 go build -o "$win_dir/max-go.exe" ./cmd/max-go
  GOOS=windows CGO_ENABLED=0 go build -o "$win_dir/max-restool.exe" ./cmd/max-restool
  GOOS=windows CGO_ENABLED=0 go build -o "$win_dir/max-convert.exe" ./cmd/max-convert
  GOOS=windows CGO_ENABLED=0 go build -o "$win_dir/max-settings.exe" ./cmd/max-settings

  copy_payload "$win_dir"
  copy_native_libs "$win_dir" "windows-x64"

  # Windows help text must be generated on Windows because these executables cannot run here.
  if [[ ! -f "$win_dir/max-go-help.txt" ]]; then
    echo "  note: no help texts in build/windows-x64, run build-windows.cmd on Windows for those"
  fi
fi

if [[ "${MAX_GO_BUILD_ONLY:-}" != "1" ]]; then
  echo
  echo "Packing release archives ..."
  pack_platform "linux-x64"
  pack_platform "windows-x64"
  rm -rf "${pack_dir:?}"
fi

echo
echo "---------------------------------------"
echo "Built: build/$platform"
if [[ "$platform" == "linux-x64" ]]; then
  echo "Cross built: build/windows-x64"
fi
echo "Build done. Run stage-linux.sh to stage."
echo
