# Data tables: ATTRIBS, CLANATRB, lang_english.ini

Three text format data tables.
The first two live in MAX.RES;
the language file is loose in the game directory.

Note: The reference implementation's current main branch replaced ATTRIBS with XOR-obfuscated JSON resources (SC_ATTRI/SC_SCHEA) that do not exist in the retail 1.0.4 data. max-go parses the original formats below.

## ATTRIBS: unit attribute table

Plain text;
`;` starts a comment line.
One row of 13 whitespace separated integers per unit type,
in the canonical unit order (COMMTWR ..
WALDO,
93 units,
the order of `assets.UnitTags`).
The unit name appears as a comment above each row but the binding is strictly by row order.

Columns,
in order:

| #  | Column | Meaning                          |
|----|--------|----------------------------------|
| 1  | Turn   | turns to build                   |
| 2  | Hits   | hit points                       |
| 3  | Armr   | armor rating                     |
| 4  | Attk   | attack rating                    |
| 5  | Move & Fire | 1 = can fire after moving without penalty |
| 6  | Sped   | movement points                  |
| 7  | Fuel   | fuel/energy reserve              |
| 8  | Rang   | attack range                     |
| 9  | Roun   | shots per turn                   |
| 10 | Scan   | scan range                       |
| 11 | Strg   | storage capacity                 |
| 12 | Ammo   | ammunition capacity              |
| 13 | Area   | blast radius                     |

Example (TANK): `4 24 10 16 0 6 80 4 2 4 0 14 0`.

## CLANATRB: clan definitions

INI style sections `[Clan A]` .. `[Clan H]`,
terminated by an `[EOD]` section.
Each clan has:

- `Name=` is the display name (e.g. `The Chosen`)
- `Text=` is the description headline
- any other `key=value` line is a unit upgrade: the key is the unit's
display name,
the value is the bonus list (e.g. `AWAC=+2 scan`).
The strings double as display text and as the source for the actual stat modifiers.

## lang_english.ini: localized strings

A `[language]` section of `key=value` lines.
Keys are 4 hex digits (the ids the code references,
e.g. `35f9=Main Menu`);
values may contain `\n` escapes for line breaks. 1135 lines in the retail file.
