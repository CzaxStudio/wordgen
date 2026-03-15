# WordGen — Professional Wordlist Generator

> For authorized penetration testing only.

## Build

```bash
go build -o wordgen .
```

## Modes & Examples

### 1. Keyword Mutation
Mutate known words with leet speak, capitalization, year appends, special chars.

```bash
./wordgen -mode=keyword -keywords=john,admin,password -leet -cap -years -special
```

Output includes: `john`, `J0hn`, `j0hn!`, `Admin2023`, `p@ssw0rd!`, etc.

---

### 2. Brute Force
Generate all combinations for a given charset and length range.

```bash
./wordgen -mode=brute -charset=alnum -min=4 -max=6 -output=brute4-6.txt
```

Charsets: `lower` | `upper` | `digits` | `alpha` | `alnum` | `all` | `<custom>`

---

### 3. Combinator
Combine two word lists with separators.

```bash
./wordgen -mode=combinator -keyfile=names.txt -append=123 -output=combined.txt
```

Generates: `john_doe`, `john-doe`, `johndoe123`, etc.

---

### 4. Pattern Mode
Use a pattern template:
- `?` = lowercase letter
- `#` = digit
- `@` = special char (`!@#$%`)
- `*` = alphanumeric
- Any other char = literal

```bash
./wordgen -mode=pattern -pattern=admin## -output=admin_pins.txt
# → admin00, admin01 ... admin99

./wordgen -mode=pattern -pattern=?#?#
# → a0a0, a0a1 ... z9z9
```

---

## All Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-mode` | `keyword` | Mode: `brute`, `keyword`, `combinator`, `pattern` |
| `-output` | `wordlist.txt` | Output file |
| `-min` | `4` | Min word length |
| `-max` | `12` | Max word length |
| `-charset` | `alnum` | Charset for brute mode |
| `-keywords` | `` | Comma-separated keywords |
| `-keyfile` | `` | File with one keyword per line |
| `-leet` | `false` | Enable leet speak |
| `-cap` | `true` | Capitalization variants |
| `-append` | `` | Append custom string to each word |
| `-prepend` | `` | Prepend custom string to each word |
| `-years` | `false` | Append years 1990–now |
| `-special` | `false` | Append common specials (`!`, `@`, `123`, etc.) |
| `-pattern` | `` | Pattern string for pattern mode |
| `-quiet` | `false` | Suppress progress output |

---

## Disclaimer
This tool is for **authorized use only**. Only run against systems you own or have explicit written permission to test.