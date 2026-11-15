# max-settings

Terminal UI for editing `max-go.json`.

Every `max-go.json` field is exposed as a numbered menu entry,
including `video.widescreen` (`auto` default / `true` / `false`: the wide 16:9 game view;
the menus stay 4:3,
and `auto` picks the aspect from the monitor at startup).

Fields with a fixed value set (booleans and choices such as the audio backend or `video.widescreen`) are shown as `<value>`.
Selecting one opens an adjuster instead of a typed prompt: the Left/Right arrow keys walk the values (A/D work too),
Enter accepts,
Esc or Q cancels.
Free text entry remains only for open fields like `player.name` and numbers.
When input is piped (no terminal),
type `<` or `>` per line to step,
an empty line to accept,
`q` to cancel,
or the literal value.

## Usage

```text
max-settings
max-settings -file <file>
```

## Parameters

```text
-file <file>
```

Uses the specified settings file instead of the default `max-go.json` next to the executable.
Without the flag,
a legacy `settings.json` next to the executable is still honored when no `max-go.json` exists.
An empty `-file=` is refused,
and so is any extra argument.

The tool does not write the file when it starts,
and not when it quits without `s`.
If the file does not exist,
the editor starts from the defaults and `s` creates the file.
Values the loader had to repair (an out of range number,
an unknown backend or language) are listed on stderr as `max-settings: repaired on load: video.window_scale was 99,
now 2`.
The repair is only in memory until you save,
so the session starts as modified.

## Interactive Commands

After startup,
the tool shows a numbered settings menu.

```text
<number>
```

Edits the selected field.
A number outside the range of its field is refused with the range,
for example `video.window_scale expects a number from 1 to 8, got 99`.
The ranges are listed in [max-go-json](max-go-json.md).

```text
s
save
```

Validates and saves the settings file.

```text
q
quit
```

Exits the settings editor.
If there are unsaved changes,
the tool asks for confirmation before discarding them.
When piped input ends,
the tool ends like `q`,
without an error.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | the editor ended normally (also at the end of piped input), or `-h` |
| `1` | a runtime failure: the settings file cannot be read or parsed, or a save failed |
| `2` | a bad command line: an unknown flag, an empty `-file=` or an extra argument |

Every message starts with `max-settings:`.
