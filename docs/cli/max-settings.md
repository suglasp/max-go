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

## Interactive Commands

After startup,
the tool shows a numbered settings menu.

```text
<number>
```

Edits the selected field.

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
