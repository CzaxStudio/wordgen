# WordGen

**A professional-grade wordlist generator built in Go for authorized penetration testers and security researchers.**

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-0d1117?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-22c55e?style=flat-square)
![Use](https://img.shields.io/badge/Use-Authorized%20Testing%20Only-dc2626?style=flat-square)

---

## Overview

WordGen is a fast, flexible, command-line wordlist generator designed for professional penetration testing engagements. It supports multiple generation strategies — from exhaustive brute-force enumeration to intelligent keyword mutation — giving security professionals a single, reliable tool for crafting targeted wordlists.

Built with Go's native concurrency and buffered I/O, WordGen is engineered for speed without sacrificing control.

---

## Legal Notice

This tool is intended **solely for authorized security testing** on systems you own or have explicit written permission to assess. Unauthorized use against third-party systems is illegal under computer fraud laws in most jurisdictions. The author accepts no liability for misuse. Always obtain written authorization before conducting any penetration test.

---

## Features

- Four distinct generation modes covering every common pen testing scenario
- Intelligent keyword mutation engine with leet speak, capitalization, year suffixes, and special character injection
- Pattern-based mask generation for targeted credential guessing
- Combinator mode for building credential pairs from multiple wordlists
- Buffered high-throughput file writing with real-time progress and words-per-second metrics
- Keyword file input support — drop in any line-delimited word list
- Full cross-platform support: Windows, Linux, macOS
- Zero external dependencies — single compiled binary, no runtime required

---

## Installation

### Requirements

- [Go 1.21 or higher](https://golang.org/dl/)

### Build from Source

```bash
git clone https://github.com/CzaxStudio/wordgen.git
cd wordgen
go build -o wordgen .
```

**Windows (PowerShell):**

```powershell
git clone https://github.com/CzaxStudio/wordgen.git
cd wordgen
go build -o wordgen.exe .
```

---

## Modes

### 1. Keyword Mutation

Mutates a set of known words using common password patterns. Ideal when you have intelligence about a target — names, company names, or known phrases.

Mutations applied:
- Leet speak substitutions (`a -> 4/@`, `e -> 3`, `o -> 0`, etc.)
- Capitalization variants (uppercase, title case, mixed)
- Year suffixes from 1990 to the current year
- Common special character and number suffixes (`!`, `@`, `123`, `$`, etc.)
- Custom prepend and append strings

```bash
# Linux / macOS
./wordgen -mode=keyword -keywords=james,techcorp -leet -cap -years -special -output=james.txt

# Windows PowerShell
./wordgen -mode=keyword -keywords="james,techcorp" -leet -cap -years -special -output=james.txt
```

---

### 2. Brute Force

Exhaustively generates every possible combination for a given character set and length range. Best for numeric PINs, short passwords, or when no prior intelligence is available.

Available charsets:

| Name | Characters |
|------|------------|
| `lower` | a-z |
| `upper` | A-Z |
| `digits` | 0-9 |
| `alpha` | a-z A-Z |
| `alnum` | a-z A-Z 0-9 |
| `all` | a-z A-Z 0-9 !@#$%^&* |
| custom | Any raw string you provide |

```bash
# All 4-digit PINs
./wordgen -mode=brute -charset=digits -min=4 -max=4 -output=pins.txt

# All 4-6 character alphanumeric combinations
./wordgen -mode=brute -charset=alnum -min=4 -max=6 -output=brute.txt
```

---

### 3. Combinator

Combines words from a list with each other using common separators. Effective for generating credential pairs such as `firstname_lastname` or `company.year`.

Separators used: ` ` (none), `_`, `-`, `.`, `123`, `!`

```bash
# From inline keywords
./wordgen -mode=combinator -keywords="john,doe,admin,root" -output=combined.txt

# From a file
./wordgen -mode=combinator -keyfile=names.txt -output=combined.txt
```

---

### 4. Pattern Mode

Generates words from a mask template. Each character in the pattern is either a literal or a wildcard that expands across a defined character class.

| Symbol | Expands To |
|--------|------------|
| `?` | All lowercase letters (a-z) |
| `#` | All digits (0-9) |
| `@` | Common special characters (!@#$%) |
| `*` | All alphanumeric characters |
| Any other character | Literal (written as-is) |

```bash
# admin00 through admin99
./wordgen -mode=pattern -pattern=admin## -output=admin_pins.txt

# All letter-digit-letter-digit combinations
./wordgen -mode=pattern -pattern=?#?# -output=pattern.txt

# Literal prefix with dynamic suffix
./wordgen -mode=pattern -pattern=root@# -output=root_variants.txt
```

---

## All Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-mode` | `keyword` | Generation mode: `brute`, `keyword`, `combinator`, `pattern` |
| `-output` | `wordlist.txt` | Path to output file |
| `-min` | `4` | Minimum word length filter |
| `-max` | `12` | Maximum word length filter |
| `-charset` | `alnum` | Character set for brute mode |
| `-keywords` | — | Comma-separated base keywords (quote in PowerShell) |
| `-keyfile` | — | Path to a newline-delimited keyword file |
| `-leet` | `false` | Enable leet speak substitutions |
| `-cap` | `true` | Enable capitalization variants |
| `-append` | — | Append a fixed string to every generated word |
| `-prepend` | — | Prepend a fixed string to every generated word |
| `-years` | `false` | Append years from 1990 to present |
| `-special` | `false` | Append common special characters and numbers |
| `-pattern` | — | Mask pattern for pattern mode |
| `-quiet` | `false` | Suppress progress output (useful for scripting) |

---

## Real-World Pen Test Example

You are conducting an authorized engagement. OSINT reveals the target user is `james`, employed at a company founded in 2019 called `techcorp`. Generate a targeted wordlist:

```bash
# Linux / macOS
./wordgen \
  -mode=keyword \
  -keywords=james,techcorp,James,TECHCORP \
  -leet -cap -years -special \
  -min=6 -max=20 \
  -output=james_targeted.txt
```

```powershell
# Windows PowerShell
./wordgen -mode=keyword -keywords="james,techcorp,James,TECHCORP" -leet -cap -years -special -min=6 -max=20 -output=james_targeted.txt
```

Feed the output into your testing tool of choice:

```bash
# SSH credential audit (authorized targets only)
hydra -l james -P james_targeted.txt ssh://TARGET_IP

# HTTP form testing via Burp Suite Intruder
# Load james_targeted.txt as the payload list
```

---

## Makefile Shortcuts

```bash
make build          # Compile binary
make test-keyword   # Run keyword mutation demo
make test-brute     # Generate all 4-digit PINs
make test-pattern   # Run pattern mode demo
make test-combo     # Run combinator demo
make clean          # Remove binary and output files
```

---

## Performance

WordGen uses a 1MB buffered writer and displays live throughput metrics during generation. Typical performance on modern hardware:

| Mode | Charset / Input | Length | Output |
|------|----------------|--------|--------|
| Brute | digits | 4-4 | 10,000 words in < 1s |
| Brute | alnum | 4-6 | ~57M words in ~30s |
| Keyword | 5 base words, all mutations | — | ~2,000 variants in < 1s |
| Pattern | `?#?#` | 4 | 67,600 words in < 1s |

---

## Project Structure

```
wordgen/
├── main.go       # Core generator — all modes, mutation engine, CLI
├── go.mod        # Go module definition
├── Makefile      # Build and test shortcuts
└── README.md     # This file
```

---

## Roadmap

- Hashcat-style rule file support (`.rule` format)
- YAML profile system for saving and reusing engagement configs
- Concurrent brute-force mode for multi-core generation
- Markov chain mode for statistically-weighted output
- Built-in deduplication and sorting pipeline

---

## Contributing

Pull requests are welcome. For major changes, open an issue first to discuss what you would like to change. Please ensure all contributions align with the responsible use policy of this project.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes (`git commit -m 'Add your feature'`)
4. Push to the branch (`git push origin feature/your-feature`)
5. Open a Pull Request

---

## License

Distributed under the MIT License. See `LICENSE` for full details.

---

## Author

Built for the security community. If this tool helped you on an authorized engagement or CTF, consider leaving a star.
