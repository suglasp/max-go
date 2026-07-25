# max-restool

M.A.X.
RES archive tool: list,
unpack,
pack.

The archive is all this tool concerns itself with.
To turn unpacked resources into images,
sounds or text,
run [max-convert](max-convert.md) over the unpacked directory.

## Usage

```text
max-restool help
max-restool -res|-file|-archive <archive.res> [-path <dir>]
max-restool list <archive.res>
max-restool unpack <archive.res> <dir>
max-restool pack <dir> <archive.res>
```

## Commands

```text
max-restool help
```

Prints command usage.

```text
max-restool -res|-file|-archive <archive.res> [-path <dir>]
```

The short form.
Naming an archive by flag unpacks it into `extracted` under the current folder,
unless `-path` names another one. `-res`,
`-file` and `-archive` are three spellings of the same flag;
which word comes to mind first varies,
so all three work.

`max-convert` reads that same `extracted` folder by default,
so the common case is two commands with nothing else to type:

```text
max-restool -res MAX.RES
max-convert res
```

```text
max-restool list <archive.res>
```

Lists entries in a RES archive.
Output includes the resource tag and byte size.

```text
max-restool unpack <archive.res> <dir>
```

Writes all entries of a RES archive into `<dir>` as raw bytes,
plus a `_res_manifest.txt` recording each entry's tag,
file name and SHA-256 hash.
The output directory is created if it does not exist.

```text
max-restool pack <dir> <archive.res>
```

Packs files from `<dir>` into a RES archive.
If `_res_manifest.txt` exists,
it is used to preserve archive order and to validate file hashes,
so an unpack followed by a pack reproduces the original archive bit for bit.
Without a manifest,
files are sorted lexically and tags are derived from filenames;
files that max-convert may have written beside the raw resources (`.bmp`,
`.png`,
`.txt`,
`.wav`) are skipped.

## Notes

`max-restool` uses subcommands and positional arguments only.
There are no dash style flags.
