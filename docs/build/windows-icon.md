# Windows executable icon

`cmd/max-go` embeds the project icon in Windows AMD64 builds through a Go linker resource object:

- `ico/max-go.ico` is this project's icon source used when regenerating the resource.
- `cmd/max-go/max_icon.rc` declares that icon as the Windows icon resource.
- `cmd/max-go/max_icon_windows_amd64.syso` is the generated COFF resource object. Go includes files with the suffix `_windows_amd64.syso` only when building `GOOS=windows GOARCH=amd64`.

The checked in `.syso` means normal builds do not need `windres`,
w64devkit,
or any other resource compiler.
A fresh clone can build the Windows executable icon as is:

```powershell
cd <repo-root>
go build -o bin\ ./cmd/max-go
```

Only regenerate the resource object after replacing `ico/max-go.ico`.
Regeneration needs GNU `windres`;
it can come from w64devkit,
MSYS2/MinGW,
or any other Windows resource compiler that can emit an x86-64 COFF object.

The repo has separate helper scripts with variables at the top for the resource compiler,
icon directory,
icon filename,
RC output,
and `.syso` output. `ICON_REF` is the path written into the RC file;
by default it is `../../ico/max-go.ico` from `cmd/max-go`.

```powershell
cd <repo-root>
.\generate-max-icon.cmd
```

```bash
cd <repo-root>
./generate-max-icon.sh
```

Set `RC_ONLY=1` in either script when you only want to rewrite `cmd/max-go/max_icon.rc` and skip the `windres` compile step.

```powershell
cd <repo-root>\cmd\max-go
windres --target=pe-x86-64 -i max_icon.rc -o max_icon_windows_amd64.syso
```

If `windres.exe` is already on `PATH`,
the package also documents the same command as a Go generate directive.
This is optional maintenance tooling and is not part of the normal build:

```powershell
cd <repo-root>
go generate ./cmd/max-go
```

The resource script is intentionally minimal:

```rc
IDI_ICON1 ICON "../../ico/max-go.ico"
```

Useful verification commands:

```powershell
cd <repo-root>
$env:GOCACHE = Join-Path $env:TEMP "max-go-gocache"
go build -o (Join-Path $env:TEMP "max-go-icon-test.exe") ./cmd/max-go

Add-Type -AssemblyName System.Drawing
$icon = [System.Drawing.Icon]::ExtractAssociatedIcon((Join-Path $env:TEMP "max-go-icon-test.exe"))
[pscustomobject]@{ Width = $icon.Width; Height = $icon.Height }
$icon.Dispose()

objdump -x .\cmd\max-go\max_icon_windows_amd64.syso | Select-String -Pattern "file format|Resource|\.rsrc"

$env:GOOS = "windows"; $env:GOARCH = "amd64"
go list -f "{{.SysoFiles}}" ./cmd/max-go
Remove-Item Env:\GOOS,Env:\GOARCH -ErrorAction SilentlyContinue
```
