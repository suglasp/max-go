"""Pack a Linux build folder into a .tar.zst with correct Unix permissions.

build-windows.cmd uses this instead of tar.exe. bsdtar reads permissions from the
filesystem, and NTFS has no execute bit, so a tarball packed on Windows comes out
with every file mode 0666 and the unpacked binaries refuse to run. Here the modes
are set explicitly: 0755 for the executables and shared libraries, 0644 for the
rest.

Usage: python pack-linux-tar.py <source-dir> <archive-path> <root-name>
"""

import os
import sys
import tarfile

from compression import zstd

EXECUTABLES = {"max-go", "max-restool", "max-convert", "max-settings"}


def is_executable(name):
    """Reports whether the archive entry should carry the execute bit."""
    return name in EXECUTABLES or name.endswith(".so") or ".so-" in name


def main():
    if len(sys.argv) != 4:
        sys.stderr.write("usage: pack-linux-tar.py <source-dir> <archive-path> <root-name>\n")
        return 2

    source_dir, archive_path, root_name = sys.argv[1], sys.argv[2], sys.argv[3]
    if not os.path.isdir(source_dir):
        sys.stderr.write("pack-linux-tar: %s is not a directory\n" % source_dir)
        return 1

    names = sorted(n for n in os.listdir(source_dir) if os.path.isfile(os.path.join(source_dir, n)))
    if not names:
        sys.stderr.write("pack-linux-tar: %s is empty\n" % source_dir)
        return 1

    os.makedirs(os.path.dirname(os.path.abspath(archive_path)), exist_ok=True)

    root = tarfile.TarInfo(root_name)
    root.type = tarfile.DIRTYPE
    root.mode = 0o755
    root.mtime = int(os.path.getmtime(source_dir))

    with zstd.ZstdFile(archive_path, "wb") as compressed:
        # Stream mode, because the zstd writer is not seekable.
        with tarfile.open(fileobj=compressed, mode="w|") as archive:
            archive.addfile(root)

            for name in names:
                path = os.path.join(source_dir, name)
                info = tarfile.TarInfo("%s/%s" % (root_name, name))
                info.size = os.path.getsize(path)
                info.mtime = int(os.path.getmtime(path))
                info.mode = 0o755 if is_executable(name) else 0o644
                # A released archive should not carry a build account's ids.
                info.uid = 0
                info.gid = 0
                info.uname = ""
                info.gname = ""

                with open(path, "rb") as handle:
                    archive.addfile(info, handle)

    print("  packed %d files into %s" % (len(names), archive_path))
    return 0


if __name__ == "__main__":
    sys.exit(main())
