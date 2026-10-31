# LANG.RES (max-go addon archive)

LANG.RES is max-go's own resource archive.
It is a plain RES archive (same header and index as `MAX.RES`, see [res.md](res.md)),
built from the text files in the repository's `langres/` folder.
It ships next to the executable, the way klei's port ships its own `PATCHES.RES`.
It is not vanilla game data and holds no retail art.

## Entries

| Tag | Source file | Content |
| --- | --- | --- |
| `LANG_EN` | `langres/lang-en-us.ini` | English UI strings, one `key=text` per line under `[language]`. |
| `UNITDESC` | `langres/unit-descriptions.txt` | The language key of each unit's description text. |

A language code maps to its tag as `LANG_` plus the part before the dash, upper case
(`en-us` -> `LANG_EN`, `de-de` -> `LANG_DE`), at most 8 characters.

## unit-descriptions.txt

One line per unit: `TAG=key`, where `TAG` is the unit's resource tag (`CONSTRCT`, `TANK`, ...)
and `key` is the language key of its description (`d28e`).
Blank lines and lines starting with `#` are ignored; tags are matched without case.
Units with no description (rubble, tapes, slabs, cones, explosions, projectiles, props) are left out.
The keys come from klei's `SC_UNITS` unit data (the `description` field, klei snapshot 23/08/26).
The text itself lives in the language file, so a translation only touches that file.

## Lookup order

Language strings, first hit wins:

1. a loose `lang-<code>.ini` in the game data folder, then next to the executable (override);
2. `LANG.RES` entry `LANG_<XX>` for the language;
3. `LANG.RES` entry `LANG_EN`;
4. the built in English fallback strings.

`LANG.RES` itself is searched in the game data folder, then next to the executable.
Unit description keys come only from `LANG.RES` `UNITDESC`; without it no description is shown.

## Building

`_res_manifest.txt` in `langres/` lists `TAG<TAB>file` lines (no hash column, so the text stays editable):

```
max-restool pack langres LANG.RES
```

The build scripts (`build-windows.cmd`, `build-linux.sh`) do this into the build folder,
and the stage scripts copy `LANG.RES` into `bin/`. See `langres/HOWTO.txt`.
