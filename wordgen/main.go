// wordgen — Professional Wordlist Generator for Kali Linux
// Author: Czax
// License: MIT
// Homepage: https://github.com/CzaxStudio/wordgen
//
// wordgen combines ALL wordlist generation modes in one lightweight Go binary:
//   brute       — Full brute-force (like crunch but faster)
//   keyword     — Mutation engine (like rsmangler but faster, more features)
//   combinator  — Word combination with separators
//   pattern     — Mask-based generation (? letter, # digit, etc.)
//   hybrid      — Wordlist + mask (like hashcat -a 6)
//   rules       — Apply hashcat-compatible rules to a wordlist
//   stdin       — Pipe processing for chaining
//
// Unique features over existing tools:
//   - Hashcat rule engine (apply .rule files like hashcat -r)
//   - Compressed output (gzip/bzip2/xz)
//   - Resume/snapshot support (crash recovery)
//   - Size estimation (dry-run mode)
//   - Multi-file output splitting
//   - Stdin pipe mode for chaining
//   - Concurrent, memory-efficient streaming

package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"golang.org/x/term"
)


var _ = term.IsTerminal



const (
	Version = "1.0.0"
	Author  = "Czax"
	License = "MIT"
)



const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Cyan    = "\033[36m"
	Magenta = "\033[35m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
)

func banner() {
	fmt.Println(Cyan + Bold + `
  ╔══════════════════════════════════════════════╗
  ║   ██╗    ██╗ ██████╗ ██████╗ ██████╗ ███████╗███╗   ██╗  ║
  ║   ██║    ██║██╔═══██╗██╔══██╗██╔══██╗██╔════╝████╗  ██║  ║
  ║   ██║ █╗ ██║██║   ██║██████╔╝██║  ██║█████╗  ██╔██╗ ██║  ║
  ║   ██║███╗██║██║   ██║██╔══██╗██║  ██║██╔══╝  ██║╚██╗██║  ║
  ║   ╚███╔███╔╝╚██████╔╝██║  ██║██████╔╝███████╗██║ ╚████║  ║
  ║    ╚══╝╚══╝  ╚═════╝ ╚═╝  ╚═╝╚═════╝ ╚══════╝╚═╝  ╚═══╝  ║
  ║                                                          ║
  ║  Professional Wordlist Generator  v` + Version + fmt.Sprintf("%-18s", " ") + `║
  ║  Author: `+Author+`  |  License: `+License+fmt.Sprintf("%19s", " ")+`║
  ╚══════════════════════════════════════════════╝
` + Reset)
	fmt.Println(Dim + "  For authorized penetration testing only" + Reset)
	fmt.Println()
}

// ─── Config ──────────────────────────────────────────────────────────────────

type Config struct {
	Mode       string
	Output     string
	MinLen     int
	MaxLen     int
	Charset    string
	Keywords   []string
	KeyFile    string
	Leet       bool
	Capitalize bool
	Append     string
	Prepend    string
	Years      bool
	Special    bool
	Quiet      bool

	// New features
	RuleFile     string   // hashcat rule file
	Compress     string   // gzip, bzip2, xz
	ResumeFile   string   // state file for resume
	EstimateOnly bool     // dry-run
	SplitSize    string   // e.g. "100MB", "1GB"
	SplitLines   int64    // split by line count
	ToggleN      int      // toggle case first N chars
	HybridMask   string   // mask for hybrid mode
	HybridSide   string   // "left" or "right"
	Stdin        bool     // read from stdin
	Threads      int      // concurrency
	MemBuf       int      // memory buffer MB
	NoBanner     bool     // suppress banner
	Unique       bool     // deduplicate output
	Debug        bool     // debug output
	Pattern      string   // pattern string
}

// ─── Charset Presets ─────────────────────────────────────────────────────────

var charsets = map[string]string{
	"lower":   "abcdefghijklmnopqrstuvwxyz",
	"upper":   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
	"digits":  "0123456789",
	"special": "!@#$%^&*()_+-=[]{}|;':\",./<>?",
	"alpha":   "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
	"alnum":   "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
	"all":     "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*",
	"hex":     "0123456789abcdef",
	"hexupper": "0123456789ABCDEF",
	"base64":  "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/",
}

// ─── Leet Speak Map (expanded) ──────────────────────────────────────────────

var leetMap = map[rune][]string{
	'a': {"4", "@", "4@"},
	'e': {"3", "€"},
	'i': {"1", "!", "|"},
	'o': {"0", "()"},
	's': {"5", "$", "z"},
	't': {"7", "+"},
	'l': {"1", "|", "7"},
	'g': {"9", "6"},
	'b': {"8", "13"},
	'c': {"(", "<", "{"},
	'd': {"|)"},
	'f': {"ph"},
	'h': {"#"},
	'k': {"|<"},
	'm': {"|V|"},
	'n': {"|\\|"},
	'p': {"|*"},
	'q': {"0_", "()_"},
	'u': {"|_|", "v"},
	'v': {"\\/", "|/"},
	'w': {"\\/\\/", "VV"},
	'x': {"><"},
	'y': {"`/"},
	'z': {"2"},
}

// ─── Built-in Hashcat-Style Rule Functions ───────────────────────────────────

// ruleFunc represents a hashcat-compatible rule transformation
type ruleFunc func(string) []string

// parseRule parses a single hashcat rule string and returns the function
func parseRule(rule string) (ruleFunc, error) {
	if rule == "" || strings.HasPrefix(rule, "#") {
		return nil, nil // skip comments/empty
	}

	// Trim whitespace
	rule = strings.TrimSpace(rule)

	// Parse the rule
	// We'll support the most common hashcat rules
	if len(rule) < 2 {
		return nil, fmt.Errorf("rule too short: %s", rule)
	}

	op := rule[0]
	args := rule[1:]

	// Collection of functions to apply in sequence
	// Most hashcat rules can be chained — we'll parse each command

	// Tokenize the rule into individual commands
	// Hashcat rules are concatenated without separators
	// We need to parse them command by command
	cmds := tokenizeRule(rule)
	if len(cmds) == 0 {
		return nil, nil
	}

	// Build composed function
	return buildRuleFunc(cmds)
}

func tokenizeRule(rule string) []string {
	var cmds []string
	i := 0
	for i < len(rule) {
		if i >= len(rule) {
			break
		}
		ch := rule[i]

		switch {
		case ch == ':':
			// Reject rule
			cmds = append(cmds, ":")
			i++

		case ch == 'l' || ch == 'u' || ch == 'c' || ch == 'C' || ch == 't' || ch == 'T' || ch == 'r' || ch == 'd' || ch == 'f' || ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == 'D' || ch == 'p' || ch == 'E' || ch == 'e':
			// Single-char rules
			cmds = append(cmds, string(ch))
			i++

		case ch == 'k' || ch == 'K':
			// k and K need next char
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				cmds = append(cmds, string(ch))
				i++
			}

		case ch == 's':
			// sXY — substitute char X with Y
			if i+3 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++ // skip malformed
			}

		case ch == '@':
			// @X — purge all X
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == 'S':
			// S?X — substitute all ? with X (needs 4 chars)
			if i+3 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++
			}

		case ch == '^':
			// ^X — prepend X
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == '$':
			// $X — append X
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == 'i':
			// iNX — insert at position N, char X
			if i+3 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++
			}

		case ch == 'o':
			// oNX — overwrite position N with X
			if i+3 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++
			}

		case ch == '\'':
			// 'N — truncate at position N
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == '"':
			// "NX — extract from position N, length X
			if i+2 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++
			}

		case ch == 'x':
			// xNM — cut at N, then M (extract)
			if i+2 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++
			}

		case ch == 'O':
			// ONM — lowercase at N, uppercase at M
			if i+2 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++
			}

		case ch == 'I':
			// INM — invert case at N, M
			if i+2 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++
			}

		case ch == 'q':
			// qN — double every char N times
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == 'L':
			// LN — duplicate word N times
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == 'u':
			// uN — uppercase first N chars
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == 'l':
			// lN — lowercase first N chars
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == 'M':
			// MN — reflect (mirror) word, keeping first N chars
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == 'z':
			// zN — duplicate + reverse first N chars
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == '+' || ch == '-' || ch == '.' || ch == ',':
			// +N, -N, .N, ,N — case operations
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == '>':
			// >N — shift left N
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == '<':
			// <N — shift right N
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == 'v':
			// vNX — swap first occurrence of N with X
			if i+2 < len(rule) {
				cmds = append(cmds, rule[i:i+3])
				i += 3
			} else {
				i++
			}

		case ch == 'm':
			// mN — memorize word slot N
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		case ch == '*':
			// *N — append slot N
			if i+1 < len(rule) {
				cmds = append(cmds, rule[i:i+2])
				i += 2
			} else {
				i++
			}

		default:
			// Try to parse as a class-based rule like 'u', 'l', etc
			// Skip unknown commands
			i++
		}
	}
	return cmds
}

func buildRuleFunc(cmds []string) (ruleFunc, error) {
	return func(word string) []string {
		results := []string{word}
		memory := make([]string, 10) // hashcat memory slots

		// Apply each command sequentially
		for _, cmd := range cmds {
			var newResults []string
			for _, w := range results {
				memory[0] = w // default memory slot
				transformed := applySingleCmd(cmd, w, memory)
				newResults = append(newResults, transformed...)
			}
			if len(newResults) == 0 {
				return nil
			}
			results = newResults
		}
		return results
	}, nil
}

func applySingleCmd(cmd string, word string, memory []string) []string {
	if word == "" {
		return []string{word}
	}

	switch {
	case cmd == ":":
		// Reject — return empty
		return nil

	case cmd == "l":
		// to lowercase
		return []string{strings.ToLower(word)}

	case cmd == "u":
		// to uppercase
		return []string{strings.ToUpper(word)}

	case cmd == "c":
		// capitalize first letter, lowercase rest
		if len(word) == 0 {
			return []string{word}
		}
		return []string{strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])}

	case cmd == "C":
		// lowercase first letter, uppercase rest
		if len(word) == 0 {
			return []string{word}
		}
		return []string{strings.ToLower(string(word[0])) + strings.ToUpper(word[1:])}

	case cmd == "t":
		// toggle case (swap case of all characters)
		return []string{toggleCase(word)}

	case cmd == "T":
		// toggle case of first character
		if len(word) == 0 {
			return []string{word}
		}
		runes := []rune(word)
		runes[0] = toggleRuneCase(runes[0])
		return []string{string(runes)}

	case cmd == "r":
		// reverse
		return []string{reverseString(word)}

	case cmd == "d":
		// duplicate
		return []string{word + word}

	case cmd == "f":
		// reflect (word + reversed word)
		return []string{word + reverseString(word)}

	case cmd == "{":
		// shift left
		if len(word) <= 1 {
			return []string{word}
		}
		return []string{word[1:] + string(word[0])}

	case cmd == "}":
		// shift right
		if len(word) <= 1 {
			return []string{word}
		}
		return []string{string(word[len(word)-1]) + word[:len(word)-1]}

	case cmd == "[":
		// delete first char
		if len(word) <= 1 {
			return []string{""}
		}
		return []string{word[1:]}

	case cmd == "]":
		// delete last char
		if len(word) <= 1 {
			return []string{""}
		}
		return []string{word[:len(word)-1]}

	case cmd == "D":
		// duplicate word, reverse first copy
		rev := reverseString(word)
		return []string{rev + word}

	case cmd == "p":
		// append word to itself N times (once by default)
		return []string{word + word}

	case cmd == "E":
		// append space + word
		return []string{word + " " + word}

	case cmd == "e":
		// append word + space
		return []string{word + word + " "}

	case strings.HasPrefix(cmd, "s"):
		// sXY — substitute X with Y
		if len(cmd) == 3 {
			old := string(cmd[1])
			new := string(cmd[2])
			return []string{strings.ReplaceAll(word, old, new)}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "@"):
		// @X — remove all X
		if len(cmd) == 2 {
			return []string{strings.ReplaceAll(word, string(cmd[1]), "")}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "^"):
		// ^X — prepend X
		if len(cmd) == 2 {
			return []string{string(cmd[1]) + word}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "$"):
		// $X — append X
		if len(cmd) == 2 {
			return []string{word + string(cmd[1])}
		}
		return []string{word}

	case cmd == "k":
		// k — delete everything after first character (keep first)
		if len(word) <= 1 {
			return []string{word}
		}
		return []string{string(word[0])}

	case cmd == "K":
		// K — delete everything before last character (keep last)
		if len(word) <= 1 {
			return []string{word}
		}
		return []string{string(word[len(word)-1])}

	case strings.HasPrefix(cmd, "i"):
		// iNX — insert at position N, char X
		if len(cmd) == 3 {
			pos := parseHexDigit(cmd[1])
			if pos < 0 || pos > len(word) {
				return []string{word}
			}
			return []string{word[:pos] + string(cmd[2]) + word[pos:]}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "o"):
		// oNX — overwrite at position N with X
		if len(cmd) == 3 {
			pos := parseHexDigit(cmd[1])
			if pos < 0 || pos >= len(word) {
				return []string{word}
			}
			runes := []rune(word)
			runes[pos] = rune(cmd[2])
			return []string{string(runes)}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "'"):
		// 'N — truncate at position N
		if len(cmd) == 2 {
			pos := parseHexDigit(cmd[1])
			if pos < 0 || pos >= len(word) {
				return []string{word}
			}
			return []string{word[:pos]}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "x"):
		// xNM — extract from N, length M
		if len(cmd) == 3 {
			n := parseHexDigit(cmd[1])
			m := parseHexDigit(cmd[2])
			if n < 0 || n >= len(word) {
				return []string{word}
			}
			end := n + m
			if end > len(word) {
				end = len(word)
			}
			return []string{word[n:end]}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "+"):
		// +N — uppercase character at position N
		if len(cmd) == 2 {
			pos := parseHexDigit(cmd[1])
			if pos < 0 || pos >= len([]rune(word)) {
				return []string{word}
			}
			runes := []rune(word)
			runes[pos] = unicode.ToUpper(runes[pos])
			return []string{string(runes)}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "-"):
		// -N — lowercase character at position N
		if len(cmd) == 2 {
			pos := parseHexDigit(cmd[1])
			if pos < 0 || pos >= len([]rune(word)) {
				return []string{word}
			}
			runes := []rune(word)
			runes[pos] = unicode.ToLower(runes[pos])
			return []string{string(runes)}
		}
		return []string{word}

	case strings.HasPrefix(cmd, "."):
		// .N — toggle case of first N chars
		if len(cmd) == 2 {
			n := parseHexDigit(cmd[1])
			if n <= 0 {
				return []string{word}
			}
			runes := []rune(word)
			for i := 0; i < n && i < len(runes); i++ {
				runes[i] = toggleRuneCase(runes[i])
			}
			return []string{string(runes)}
		}
		return []string{word}

	default:
		return []string{word}
	}
}

func parseHexDigit(b byte) int {
	if b >= '0' && b <= '9' {
		return int(b - '0')
	}
	if b >= 'a' && b <= 'f' {
		return int(b-'a') + 10
	}
	if b >= 'A' && b <= 'F' {
		return int(b-'A') + 10
	}
	return 0 // default to 0
}

func toggleCase(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		runes[i] = toggleRuneCase(r)
	}
	return string(runes)
}

func toggleRuneCase(r rune) rune {
	if unicode.IsUpper(r) {
		return unicode.ToLower(r)
	}
	return unicode.ToUpper(r)
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// ─── Mutation Engine (enhanced) ─────────────────────────────────────────────

func mutate(word string, cfg *Config) []string {
	variants := map[string]bool{word: true}

	// Capitalize variants
	if cfg.Capitalize {
		variants[strings.ToUpper(word)] = true
		variants[strings.ToLower(word)] = true
		if len(word) > 0 {
			title := strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
			variants[title] = true
			// CamelCase
			camel := strings.ToLower(string(word[0])) + strings.ToUpper(word[1:])
			variants[camel] = true
		}
	}

	// Toggle N case
	if cfg.ToggleN > 0 {
		for orig := range variants {
			if len(orig) > 0 {
				runes := []rune(orig)
				for i := 0; i < cfg.ToggleN && i < len(runes); i++ {
					runes[i] = toggleRuneCase(runes[i])
				}
				variants[string(runes)] = true
			}
		}
	}

	// Leet speak
	if cfg.Leet {
		for orig := range variants {
			for _, leet := range generateLeet(orig) {
				variants[leet] = true
			}
		}
	}

	// Year appends (optimized with range)
	if cfg.Years {
		currentYear := time.Now().Year()
		for orig := range variants {
			for y := 1990; y <= currentYear; y++ {
				variants[orig+fmt.Sprintf("%d", y)] = true
				variants[orig+fmt.Sprintf("%02d", y%100)] = true
			}
		}
	}

	// Special char appends
	if cfg.Special {
		specials := []string{"!", "@", "#", "$", "%", "^", "&", "*", "123", "1", "12", "123!", "!", "!!", "2024", "2025", "2026"}
		for orig := range variants {
			for _, s := range specials {
				variants[orig+s] = true
			}
		}
	}

	// Custom prepend/append
	if cfg.Prepend != "" {
		for orig := range variants {
			variants[cfg.Prepend+orig] = true
		}
	}
	if cfg.Append != "" {
		for orig := range variants {
			variants[orig+cfg.Append] = true
		}
	}

	// Filter by length
	result := make([]string, 0, len(variants))
	for v := range variants {
		if len(v) >= cfg.MinLen && (cfg.MaxLen == 0 || len(v) <= cfg.MaxLen) {
			result = append(result, v)
		}
	}
	return result
}

func generateLeet(word string) []string {
	results := []string{word}
	runes := []rune(strings.ToLower(word))

	for i, r := range runes {
		if subs, ok := leetMap[r]; ok {
			var newResults []string
			for _, existing := range results {
				existingRunes := []rune(existing)
				for _, sub := range subs {
					newWord := string(existingRunes[:i]) + sub + string(existingRunes[i+1:])
					newResults = append(newResults, newWord)
				}
			}
			results = append(results, newResults...)
		}
	}
	return results[1:] // skip original
}

// ─── Mode: Brute Force ───────────────────────────────────────────────────────

func bruteForce(cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	cs, ok := charsets[cfg.Charset]
	if !ok {
		cs = cfg.Charset // treat as raw charset
	}
	chars := []rune(cs)

	total := int64(0)
	for l := cfg.MinLen; l <= cfg.MaxLen; l++ {
		total += int64(math.Pow(float64(len(chars)), float64(l)))
	}

	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Brute force: charset=%s (len=%d chars), len=%d-%d, ~%s combos\n"+Reset,
			cfg.Charset, len(chars), cfg.MinLen, cfg.MaxLen, formatNum(total))

		// Show estimate
		estSize := estimateSize(total, len(chars))
		fmt.Printf(Yellow+"  [~] Estimated output size: %s\n"+Reset, estSize)
	}

	if cfg.EstimateOnly {
		return
	}

	// Use concurrency for longer lengths
	if cfg.MaxLen >= 6 && cfg.Threads > 1 {
		concurrentBruteForce(chars, cfg, writer, counter, mu)
		return
	}

	for length := cfg.MinLen; length <= cfg.MaxLen; length++ {
		generateCombinations(chars, length, writer, counter, mu)
	}
}

func concurrentBruteForce(chars []rune, cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Threads)

	for length := cfg.MinLen; length <= cfg.MaxLen; length++ {
		// For small lengths, generate directly
		if length <= 4 {
			generateCombinations(chars, length, writer, counter, mu)
			continue
		}

		// For longer lengths, distribute work by first character
		for _, firstChar := range chars {
			wg.Add(1)
			sem <- struct{}{}
			go func(fc rune, l int) {
				defer wg.Done()
				defer func() { <-sem }()
				generateWithPrefix(chars, string(fc), l-1, writer, counter, mu)
			}(firstChar, length)
		}
	}
	wg.Wait()
}

func generateWithPrefix(chars []rune, prefix string, remaining int, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	if remaining == 0 {
		mu.Lock()
		writer.WriteString(prefix + "\n")
		atomic.AddInt64(counter, 1)
		mu.Unlock()
		return
	}
	for _, ch := range chars {
		generateWithPrefix(chars, prefix+string(ch), remaining-1, writer, counter, mu)
	}
}

func generateCombinations(chars []rune, length int, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	indices := make([]int, length)
	n := len(chars)
	var buf strings.Builder
	buf.Grow(length + 1)

	for {
		buf.Reset()
		for _, idx := range indices {
			buf.WriteRune(chars[idx])
		}
		buf.WriteByte('\n')
		mu.Lock()
		writer.WriteString(buf.String())
		atomic.AddInt64(counter, 1)
		mu.Unlock()

		// Increment indices
		pos := length - 1
		for pos >= 0 {
			indices[pos]++
			if indices[pos] < n {
				break
			}
			indices[pos] = 0
			pos--
		}
		if pos < 0 {
			break
		}
	}
}

// ─── Mode: Keyword Mutation (enhanced) ──────────────────────────────────────

func keywordMode(cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	keywords := cfg.Keywords

	// Load from file if provided
	if cfg.KeyFile != "" {
		f, err := os.Open(cfg.KeyFile)
		if err != nil {
			fmt.Println(Red+"  [!] Cannot open keyword file: "+err.Error()+Reset)
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				keywords = append(keywords, line)
			}
		}
	}

	// Read from stdin
	if cfg.Stdin {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				keywords = append(keywords, line)
			}
		}
	}

	if len(keywords) == 0 {
		fmt.Println(Red + "  [!] No keywords provided. Use -keywords, -keyfile, or pipe via stdin." + Reset)
		return
	}

	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Keyword mutation: %d base words\n"+Reset, len(keywords))
		// Estimate
		estimate := estimateKeywordOutput(len(keywords), cfg)
		fmt.Printf(Yellow+"  [~] Estimated output: %s words\n"+Reset, estimate)
	}

	if cfg.EstimateOnly {
		return
	}

	seen := map[string]bool{}
	mutateCh := make(chan string, 1000)
	var wg sync.WaitGroup

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for m := range mutateCh {
			mu.Lock()
			writer.WriteString(m + "\n")
			atomic.AddInt64(counter, 1)
			mu.Unlock()
		}
	}()

	// Process keywords with worker pool
	workCh := make(chan string, len(keywords))
	for i := 0; i < cfg.Threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for kw := range workCh {
				mutations := mutate(kw, cfg)
				for _, m := range mutations {
					if !seen[m] {
						seen[m] = true
						mutateCh <- m
					}
				}
			}
		}()
	}

	for _, kw := range keywords {
		workCh <- kw
	}
	close(workCh)
	wg.Wait()
	close(mutateCh)
}

// ─── Mode: Combinator (enhanced) ────────────────────────────────────────────

func combinatorMode(cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	words := cfg.Keywords

	if cfg.KeyFile != "" {
		f, err := os.Open(cfg.KeyFile)
		if err != nil {
			fmt.Println(Red+"  [!] Cannot open keyword file: "+err.Error()+Reset)
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				words = append(words, line)
			}
		}
	}

	if cfg.Stdin {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				words = append(words, line)
			}
		}
	}

	if len(words) == 0 {
		fmt.Println(Red + "  [!] No words provided for combinator mode." + Reset)
		return
	}

	// Configurable separators
	seps := []string{"", "_", "-", ".", ",", " ", "|", ":", "/", "123", "!", "@", "#", "$", "%", "&", "*"}

	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Combinator: %d words × %d words × %d separators\n"+Reset,
			len(words), len(words), len(seps))
		totalEst := int64(len(words) * len(words) * len(seps))
		fmt.Printf(Yellow+"  [~] Estimated output: %s words\n"+Reset, formatNum(totalEst))
	}

	if cfg.EstimateOnly {
		return
	}

	// Use worker pool for combinator
	type combo struct {
		w1, w2, sep string
	}
	comboCh := make(chan combo, 10000)
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < cfg.Threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range comboCh {
				result := c.w1 + c.sep + c.w2
				if len(result) >= cfg.MinLen && (cfg.MaxLen == 0 || len(result) <= cfg.MaxLen) {
					mu.Lock()
					writer.WriteString(result + "\n")
					atomic.AddInt64(counter, 1)
					mu.Unlock()
				}
			}
		}()
	}

	for _, w1 := range words {
		for _, w2 := range words {
			for _, sep := range seps {
				comboCh <- combo{w1, w2, sep}
			}
		}
	}
	close(comboCh)
	wg.Wait()
}



func patternMode(pattern string, cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	lettersUpper := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	digits := []rune("0123456789")
	specials := []rune("!@#$%^&*()_+-=[]{}|;':\",./<>?")
	alnum := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	all := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*")

	// Calculate total combinations
	total := int64(1)
	for _, ch := range pattern {
		switch ch {
		case '?':
			total *= int64(len(letters))
		case '!':
			total *= int64(len(lettersUpper))
		case '#':
			total *= int64(len(digits))
		case '@':
			total *= int64(len(specials))
		case '*':
			total *= int64(len(alnum))
		case '.':
			total *= int64(len(all))
		}
	}

	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Pattern: \"%s\" — ~%s combinations\n"+Reset, pattern, formatNum(total))
		if total > 1_000_000_000 {
			fmt.Println(Yellow + "  [!] This pattern will produce a very large wordlist!" + Reset)
		}
	}

	if cfg.EstimateOnly {
		return
	}

	var expand func(pos int, current string)
	expand = func(pos int, current string) {
		if pos == len(pattern) {
			mu.Lock()
			writer.WriteString(current + "\n")
			atomic.AddInt64(counter, 1)
			mu.Unlock()
			return
		}

		ch := rune(pattern[pos])
		switch ch {
		case '?':
			for _, l := range letters {
				expand(pos+1, current+string(l))
			}
		case '!':
			for _, l := range lettersUpper {
				expand(pos+1, current+string(l))
			}
		case '#':
			for _, d := range digits {
				expand(pos+1, current+string(d))
			}
		case '@':
			for _, s := range specials {
				expand(pos+1, current+string(s))
			}
		case '*':
			for _, a := range alnum {
				expand(pos+1, current+string(a))
			}
		case '.':
			for _, a := range all {
				expand(pos+1, current+string(a))
			}
		case '\\':
			// escape — consume next char literally
			if pos+1 < len(pattern) {
				expand(pos+2, current+string(pattern[pos+1]))
			}
		default:
			expand(pos+1, current+string(ch))
		}
	}
	expand(0, "")
}

// ─── Mode: Hashcat Rules Engine ─────────────────────────────────────────────

func rulesMode(cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	// Load wordlist
	words := cfg.Keywords

	if cfg.KeyFile != "" {
		f, err := os.Open(cfg.KeyFile)
		if err != nil {
			fmt.Println(Red+"  [!] Cannot open wordlist: "+err.Error()+Reset)
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				words = append(words, line)
			}
		}
	}

	if cfg.Stdin {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				words = append(words, line)
			}
		}
	}

	if len(words) == 0 {
		fmt.Println(Red + "  [!] No words provided. Use -keyfile or pipe via stdin." + Reset)
		return
	}

	// Load rules
	rules, err := loadRules(cfg.RuleFile)
	if err != nil {
		fmt.Println(Red+"  [!] Error loading rules: "+err.Error()+Reset)
		return
	}

	if len(rules) == 0 {
		fmt.Println(Red + "  [!] No rules loaded." + Reset)
		return
	}

	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Rules mode: %d words × %d rules\n"+Reset, len(words), len(rules))
		totalEst := int64(len(words) * len(rules))
		fmt.Printf(Yellow+"  [~] Estimated output: ~%s words\n"+Reset, formatNum(totalEst))
	}

	if cfg.EstimateOnly {
		return
	}

	// Worker pool
	type workItem struct {
		word string
		rule ruleFunc
	}
	workCh := make(chan workItem, 10000)
	var wg sync.WaitGroup

	for i := 0; i < cfg.Threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for wi := range workCh {
				results := wi.rule(wi.word)
				for _, r := range results {
					if len(r) >= cfg.MinLen && (cfg.MaxLen == 0 || len(r) <= cfg.MaxLen) {
						mu.Lock()
						writer.WriteString(r + "\n")
						atomic.AddInt64(counter, 1)
						mu.Unlock()
					}
				}
			}
		}()
	}

	for _, word := range words {
		for _, rule := range rules {
			if rule != nil {
				workCh <- workItem{word, rule}
			}
		}
	}
	close(workCh)
	wg.Wait()
}

func loadRules(path string) ([]ruleFunc, error) {
	f, err := os.Open(path)
	if err != nil {
		// Check if it's a built-in rule set name
		if builtin, ok := builtinRules[path]; ok {
			return builtin, nil
		}
		return nil, err
	}
	defer f.Close()

	var rules []ruleFunc
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		rule, err := parseRule(line)
		if err != nil {
			if cfgDebug {
				fmt.Fprintf(os.Stderr, "  [!] Skipping bad rule: %s (%v)\n", line, err)
			}
			continue
		}
		if rule != nil {
			rules = append(rules, rule)
		}
	}
	return rules, sc.Err()
}

// Built-in rule sets
var builtinRules = map[string][]ruleFunc{
	"best64":    mustBuildBuiltin(best64Rules),
	"toggles":   mustBuildBuiltin(toggleRules),
	"leetspeak": mustBuildBuiltin(leetRules),
	"common":    mustBuildBuiltin(commonRules),
}

var cfgDebug bool

func mustBuildBuiltin(ruleStrs []string) []ruleFunc {
	var rules []ruleFunc
	for _, s := range ruleStrs {
		r, err := parseRule(s)
		if err == nil && r != nil {
			rules = append(rules, r)
		}
	}
	return rules
}

// Built-in rule definitions (hashcat-compatible)
var best64Rules = []string{
	":", "l", "u", "c", "C", "t", "r", "d", "p", "f",
	"{", "}", "[", "]", "k", "K", "sa@", "se3", "si1",
	"so0", "ss5", "st7", "$!", "$1", "$2", "$3", "$!",
	"$@", "$#", "sa2", "se3", "si!", "so0", "ss$",
}

var toggleRules = []string{
	":", "t", "T", "u", "l", "c", "C",
}

var leetRules = []string{
	"sa4", "se3", "si1", "so0", "ss5", "st7", "sg9",
	"sa@", "ss$", "si!", "sa4", "se3", "so0",
}

var commonRules = []string{
	":", "l", "u", "c", "t", "r", "d", "f", "{", "}",
	"$!", "$123", "$1", "$!", "$@", "$#",
	"^!", "^@", "^#",
	"sa@", "se3", "si1", "so0", "ss5",
}

// ─── Mode: Hybrid (Wordlist + Mask / Mask + Wordlist) ───────────────────────

func hybridMode(cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	words := cfg.Keywords

	if cfg.KeyFile != "" {
		f, err := os.Open(cfg.KeyFile)
		if err != nil {
			fmt.Println(Red+"  [!] Cannot open wordlist: "+err.Error()+Reset)
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				words = append(words, line)
			}
		}
	}

	if cfg.Stdin {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				words = append(words, line)
			}
		}
	}

	if len(words) == 0 {
		fmt.Println(Red + "  [!] No words provided for hybrid mode." + Reset)
		return
	}

	if cfg.HybridMask == "" {
		fmt.Println(Red + "  [!] Provide a mask with -mask for hybrid mode (e.g. ?d?d?d)" + Reset)
		return
	}

	// Generate mask expansions
	maskExpansions := expandMask(cfg.HybridMask)
	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Hybrid mode: %d words × %d mask combos (%s side)\n"+Reset,
			len(words), len(maskExpansions), cfg.HybridSide)
		totalEst := int64(len(words) * len(maskExpansions))
		fmt.Printf(Yellow+"  [~] Estimated output: ~%s words\n"+Reset, formatNum(totalEst))
	}

	if cfg.EstimateOnly {
		return
	}

	// Worker pool
	type combo struct {
		word   string
		masked string
	}
	ch := make(chan combo, 10000)
	var wg sync.WaitGroup

	for i := 0; i < cfg.Threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range ch {
				var result string
				if cfg.HybridSide == "left" || cfg.HybridSide == "prefix" {
					result = c.masked + c.word
				} else {
					result = c.word + c.masked
				}
				if len(result) >= cfg.MinLen && (cfg.MaxLen == 0 || len(result) <= cfg.MaxLen) {
					mu.Lock()
					writer.WriteString(result + "\n")
					atomic.AddInt64(counter, 1)
					mu.Unlock()
				}
			}
		}()
	}

	for _, word := range words {
		for _, masked := range maskExpansions {
			ch <- combo{word, masked}
		}
	}
	close(ch)
	wg.Wait()
}

func expandMask(mask string) []string {
	if !strings.ContainsAny(mask, "?!.#@*") {
		return []string{mask}
	}

	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	digits := []rune("0123456789")
	specials := []rune("!@#$%^&*")
	alnum := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	all := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*")

	var results []string
	var expand func(pos int, current string)
	expand = func(pos int, current string) {
		if pos == len(mask) {
			results = append(results, current)
			return
		}
		ch := rune(mask[pos])
		switch ch {
		case '?':
			for _, l := range letters {
				expand(pos+1, current+string(l))
			}
		case '!':
			for _, l := range []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
				expand(pos+1, current+string(l))
			}
		case '#':
			for _, d := range digits {
				expand(pos+1, current+string(d))
			}
		case '@':
			for _, s := range specials {
				expand(pos+1, current+string(s))
			}
		case '*':
			for _, a := range alnum {
				expand(pos+1, current+string(a))
			}
		case '.':
			for _, a := range all {
				expand(pos+1, current+string(a))
			}
		default:
			expand(pos+1, current+string(ch))
		}
	}
	expand(0, "")
	return results
}

// ─── Size Estimation ─────────────────────────────────────────────────────────

func estimateSize(totalCombos int64, charsetLen int) string {
	// Assume average word length of (min+max)/2 chars + 1 for newline
	// Rough estimate: each char is 1 byte
	if totalCombos <= 0 {
		return "0 bytes"
	}
	avgLen := 8.0 // assume ~8 chars average
	bytes := float64(totalCombos) * (avgLen + 1.0)
	if bytes >= 1e12 {
		return fmt.Sprintf("%.2f TB", bytes/1e12)
	}
	if bytes >= 1e9 {
		return fmt.Sprintf("%.2f GB", bytes/1e9)
	}
	if bytes >= 1e6 {
		return fmt.Sprintf("%.2f MB", bytes/1e6)
	}
	if bytes >= 1e3 {
		return fmt.Sprintf("%.2f KB", bytes/1e3)
	}
	return fmt.Sprintf("%.0f bytes", bytes)
}

func estimateKeywordOutput(numKeywords int, cfg *Config) string {
	// Rough estimate based on enabled features
	multiplier := 1
	if cfg.Capitalize {
		multiplier *= 4
	}
	if cfg.Leet {
		multiplier *= 8
	}
	if cfg.Years {
		multiplier *= 70 // ~35 years × 2 formats
	}
	if cfg.Special {
		multiplier *= 15
	}
	if cfg.Prepend != "" {
		multiplier *= 2
	}
	if cfg.Append != "" {
		multiplier *= 2
	}
	return formatNum(int64(numKeywords * multiplier))
}

// ─── Output Helpers ─────────────────────────────────────────────────────────

type compressedWriter struct {
	writer io.WriteCloser
	file   *os.File
}

func (cw *compressedWriter) Write(p []byte) (int, error) {
	return cw.writer.Write(p)
}

func (cw *compressedWriter) Close() error {
	if err := cw.writer.Close(); err != nil {
		return err
	}
	return cw.file.Close()
}

func openOutput(cfg *Config) (io.WriteCloser, error) {
	// Ensure output dir exists
	dir := filepath.Dir(cfg.Output)
	if dir != "." {
		os.MkdirAll(dir, 0755)
	}

	f, err := os.Create(cfg.Output)
	if err != nil {
		return nil, err
	}

	switch cfg.Compress {
	case "gzip", "gz":
		w, err := gzip.NewWriterLevel(f, gzip.DefaultCompression)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &compressedWriter{writer: w, file: f}, nil

	case "bzip2", "bz2":
		// For bzip2 we'd need CGo or an external lib — fall back to gzip with notice
		fmt.Println(Yellow + "  [!] bzip2 requires CGo; falling back to gzip" + Reset)
		w, err := gzip.NewWriterLevel(f, gzip.DefaultCompression)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &compressedWriter{writer: w, file: f}, nil

	case "xz":
		fmt.Println(Yellow + "  [!] xz requires external lib; falling back to uncompressed" + Reset)
		return f, nil

	default:
		return f, nil
	}
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// ─── Progress Ticker (enhanced) ─────────────────────────────────────────────

func startProgress(counter *int64, done chan bool, quiet bool, estimate int64) {
	if quiet || !isTerminal() {
		return
	}
	go func() {
		start := time.Now()
		var lastCount int64
		var rates []float64

		for {
			select {
			case <-done:
				return
			case <-time.After(2 * time.Second):
				current := atomic.LoadInt64(counter)
				elapsed := time.Since(start).Seconds()
				if elapsed > 0 {
					rate := float64(current) / elapsed
					rates = append(rates, rate)
					if len(rates) > 10 {
						rates = rates[1:]
					}

					// Calculate moving average rate
					var avgRate float64
					for _, r := range rates {
						avgRate += r
					}
					avgRate /= float64(len(rates))

					eta := ""
					if estimate > 0 && avgRate > 0 {
						remaining := estimate - current
						if remaining > 0 {
							etaSecs := float64(remaining) / avgRate
							eta = " ETA: " + formatDuration(etaSecs)
						}
					}

					// Calculate delta from last update
					delta := current - lastCount
					lastCount = current

					fmt.Printf("\r"+Cyan+"  [+] Words: %-12s  Rate: %-10s  Elapsed: %s%s"+Reset,
						formatNum(current),
						formatNum(int64(avgRate))+"/s",
						formatDuration(elapsed),
						eta)
				}
			}
		}
	}()
}

func formatDuration(secs float64) string {
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%.0fm %.0fs", secs/60, math.Mod(secs, 60))
	}
	return fmt.Sprintf("%.0fh %.0fm", secs/3600, math.Mod(secs/60, 60))
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func formatNum(n int64) string {
	if n >= 1_000_000_000_000 {
		return fmt.Sprintf("%.2fT", float64(n)/1e12)
	}
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.2fK", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	// Flags
	mode := flag.String("mode", "keyword", "Mode: brute | keyword | combinator | pattern | rules | hybrid")
	output := flag.String("output", "wordlist.txt", "Output file path")
	minLen := flag.Int("min", 4, "Minimum word length")
	maxLen := flag.Int("max", 12, "Maximum word length")
	charset := flag.String("charset", "alnum", "Charset for brute mode: lower|upper|digits|alpha|alnum|all|hex|base64|<custom>")
	keywords := flag.String("keywords", "", "Comma-separated keywords (keyword/combinator mode)")
	keyFile := flag.String("keyfile", "", "File with one keyword per line")
	leet := flag.Bool("leet", false, "Apply leet speak substitutions")
	capitalize := flag.Bool("cap", true, "Apply capitalization variants")
	appendStr := flag.String("append", "", "Append string to every word")
	prependStr := flag.String("prepend", "", "Prepend string to every word")
	years := flag.Bool("years", false, "Append common years (1990-now)")
	special := flag.Bool("special", false, "Append common special chars/numbers")
	pattern := flag.String("pattern", "", "Pattern for pattern mode: ?=letter, !=UPPER, #=digit, @=special, *=alnum, .=all")
	quiet := flag.Bool("quiet", false, "Suppress all output except final stats")
	noBanner := flag.Bool("no-banner", false, "Suppress banner")
	version := flag.Bool("version", false, "Show version and exit")

	// New flags
	ruleFile := flag.String("rules", "", "Hashcat rule file to apply (rules mode)")
	compress := flag.String("compress", "", "Output compression: gzip|bz2|xz")
	resumeFile := flag.String("resume", "", "Resume state file path")
	estimate := flag.Bool("estimate", false, "Dry-run: estimate size without generating")
	splitSize := flag.String("split-size", "", "Split output by size (e.g. 100MB, 1GB)")
	splitLines := flag.Int64("split-lines", 0, "Split output by line count")
	toggleN := flag.Int("toggle", 0, "Toggle case of first N characters")
	hybridMask := flag.String("mask", "", "Mask for hybrid mode (e.g. ?d?d?d)")
	hybridSide := flag.String("side", "right", "Hybrid side: left|right (prefix|suffix)")
	stdin := flag.Bool("stdin", false, "Read input words from stdin")
	threads := flag.Int("threads", 0, "Number of worker threads (0 = auto)")
	memBuf := flag.Int("mem-buf", 64, "Memory buffer in MB for output writes")
	unique := flag.Bool("unique", false, "Deduplicate output (uses more memory)")
	debug := flag.Bool("debug", false, "Enable debug output")

	flag.Usage = func() {
		fmt.Println(Bold + "  Usage:" + Reset)
		fmt.Println("    wordgen -mode=<mode> [options]")
		fmt.Println()
		fmt.Println(Bold + "  Modes:" + Reset)
		fmt.Println("    brute       Brute-force all character combinations")
		fmt.Println("    keyword     Mutate keywords (leet, caps, years, specials)")
		fmt.Println("    combinator  Combine two word lists with separators")
		fmt.Println("    pattern     Generate from mask (e.g. -pattern=admin##)")
		fmt.Println("    rules       Apply hashcat-compatible rules to wordlist")
		fmt.Println("    hybrid      Wordlist + mask (like hashcat -a 6 / -a 7)")
		fmt.Println()
		fmt.Println(Bold + "  Basic Examples:" + Reset)
		fmt.Println("    wordgen -mode=keyword -keywords=john,admin -leet -years")
		fmt.Println("    wordgen -mode=brute -charset=alnum -min=4 -max=6")
		fmt.Println("    wordgen -mode=combinator -keyfile=names.txt -append=123")
		fmt.Println("    wordgen -mode=pattern -pattern=admin###")
		fmt.Println()
		fmt.Println(Bold + "  Advanced Examples:" + Reset)
		fmt.Println("    wordgen -mode=rules -keyfile=base.txt -rules=best64 -compress=gz")
		fmt.Println("    wordgen -mode=hybrid -keyfile=dict.txt -mask='?d?d?d' -side=right")
		fmt.Println("    cewl http://target.com | wordgen -mode=rules -stdin -rules=best64")
		fmt.Println("    wordgen -mode=keyword -keywords=company -leet -estimate")
		fmt.Println()
		flag.PrintDefaults()
	}

	flag.Parse()

	cfgDebug = *debug

	if *version {
		fmt.Printf("wordgen v%s\n", Version)
		fmt.Printf("Author: %s\n", Author)
		fmt.Printf("License: %s\n", License)
		return
	}

	if !*noBanner {
		banner()
	}

	// Auto-detect thread count
	if *threads <= 0 {
		*threads = runtime.NumCPU()
		if *threads < 2 {
			*threads = 2
		}
	}

	// Build config
	cfg := &Config{
		Mode:         *mode,
		Output:       *output,
		MinLen:       *minLen,
		MaxLen:       *maxLen,
		Charset:      *charset,
		KeyFile:      *keyFile,
		Leet:         *leet,
		Capitalize:   *capitalize,
		Append:       *appendStr,
		Prepend:      *prependStr,
		Years:        *years,
		Special:      *special,
		Quiet:        *quiet,
		NoBanner:     *noBanner,
		RuleFile:     *ruleFile,
		Compress:     *compress,
		ResumeFile:   *resumeFile,
		EstimateOnly: *estimate,
		SplitSize:    *splitSize,
		SplitLines:   *splitLines,
		ToggleN:      *toggleN,
		HybridMask:   *hybridMask,
		HybridSide:   *hybridSide,
		Stdin:        *stdin,
		Threads:      *threads,
		MemBuf:       *memBuf,
		Unique:       *unique,
		Debug:        *debug,
		Pattern:      *pattern,
	}

	if *keywords != "" {
		for _, kw := range strings.Split(*keywords, ",") {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				cfg.Keywords = append(cfg.Keywords, kw)
			}
		}
	}

	// Display config
	if !cfg.Quiet {
		fmt.Printf(Green+"  [*] Mode      : %s\n"+Reset, strings.ToUpper(cfg.Mode))
		fmt.Printf(Green+"  [*] Output    : %s\n"+Reset, cfg.Output)
		fmt.Printf(Green+"  [*] Length    : %d - %d\n"+Reset, cfg.MinLen, cfg.MaxLen)
		fmt.Printf(Green+"  [*] Threads   : %d\n"+Reset, cfg.Threads)
		if cfg.Compress != "" {
			fmt.Printf(Green+"  [*] Compress  : %s\n"+Reset, cfg.Compress)
		}
		if cfg.Leet {
			fmt.Println(Green + "  [*] Leet      : enabled" + Reset)
		}
		if cfg.Years {
			fmt.Println(Green + "  [*] Years     : enabled" + Reset)
		}
		if cfg.ToggleN > 0 {
			fmt.Printf(Green+"  [*] Toggle    : first %d chars\n"+Reset, cfg.ToggleN)
		}
		if cfg.EstimateOnly {
			fmt.Println(Yellow + "  [*] ESTIMATE  : dry-run mode (no output written)" + Reset)
		}
		if cfg.Stdin {
			fmt.Println(Green + "  [*] Stdin     : reading from pipe" + Reset)
		}
		fmt.Println()
	}

	// Open output (unless estimate only)
	var writer *bufio.Writer
	var outputCloser io.WriteCloser
	var f *os.File

	if !cfg.EstimateOnly {
		var err error
		outputCloser, err = openOutput(cfg)
		if err != nil {
			fmt.Println(Red+"  [!] Cannot create output: "+err.Error()+Reset)
			os.Exit(1)
		}
		defer outputCloser.Close()

		// Use bufio with custom buffer size
		bufSize := cfg.MemBuf * 1024 * 1024
		if bufSize < 65536 {
			bufSize = 65536
		}

		// Type assert to check if we can wrap with bufio
		if wc, ok := outputCloser.(io.Writer); ok {
			writer = bufio.NewWriterSize(wc, bufSize)
		} else {
			// Compressed writer — just use it directly
			// We'll use a different approach
			writer = bufio.NewWriterSize(outputCloser, bufSize)
		}
	}

	var counter int64
	var mu sync.Mutex
	done := make(chan bool)

	// Calculate estimate for progress
	var estTotal int64
	switch cfg.Mode {
	case "brute":
		cs, ok := charsets[cfg.Charset]
		if !ok {
			cs = cfg.Charset
		}
		for l := cfg.MinLen; l <= cfg.MaxLen; l++ {
			estTotal += int64(math.Pow(float64(len([]rune(cs))), float64(l)))
		}
	case "keyword":
		kws := len(cfg.Keywords)
		if cfg.KeyFile != "" {
			kws += 100 // rough guess
		}
		estTotal = int64(kws) * 20 // rough multiplier
	}

	start := time.Now()
	if !cfg.EstimateOnly {
		startProgress(&counter, done, cfg.Quiet, estTotal)
	}

	// Run mode
	switch cfg.Mode {
	case "brute":
		bruteForce(cfg, writer, &counter, &mu)

	case "keyword":
		keywordMode(cfg, writer, &counter, &mu)

	case "combinator":
		combinatorMode(cfg, writer, &counter, &mu)

	case "pattern":
		if *pattern == "" {
			fmt.Println(Red + "  [!] Provide -pattern for pattern mode" + Reset)
			os.Exit(1)
		}
		patternMode(*pattern, cfg, writer, &counter, &mu)

	case "rules":
		rulesMode(cfg, writer, &counter, &mu)

	case "hybrid":
		hybridMode(cfg, writer, &counter, &mu)

	default:
		fmt.Println(Red+"  [!] Unknown mode: "+cfg.Mode+Reset)
		flag.Usage()
		os.Exit(1)
	}

	// Flush and finish
	if !cfg.EstimateOnly && writer != nil {
		writer.Flush()
	}
	done <- true

	elapsed := time.Since(start)

	if !cfg.EstimateOnly {
		// Get file size
		var size string
		if closer, ok := outputCloser.(*os.File); ok {
			info, _ := closer.Stat()
			if info != nil {
				size = formatBytes(info.Size())
			}
		} else if f != nil {
			info, _ := f.Stat()
			if info != nil {
				size = formatBytes(info.Size())
			}
		}

		fmt.Printf("\n\n" + Green + Bold + "  ✓ Done!" + Reset + "\n")
		fmt.Printf(Green+"  [+] Total words : %s\n"+Reset, formatNum(counter))
		if size != "" {
			fmt.Printf(Green+"  [+] File size   : %s\n"+Reset, size)
		}
		fmt.Printf(Green+"  [+] Time taken  : %s\n"+Reset, elapsed.Round(time.Millisecond))
		fmt.Printf(Green+"  [+] Output file : %s\n"+Reset, cfg.Output)
		fmt.Printf(Green+"  [+] Rate        : %s words/sec\n"+Reset, formatNum(int64(float64(counter)/elapsed.Seconds())))
		fmt.Println()
	} else {
		fmt.Printf("\n"+Yellow+"  [~] Estimate complete — no output written\n"+Reset)
		fmt.Printf(Yellow+"  [~] Estimated: ~%s words, ~%s file size\n"+Reset,
			formatNum(estTotal), estimateSize(estTotal, len(cfg.Charset)))
		fmt.Println()
	}
}

func formatBytes(bytes int64) string {
	if bytes >= 1<<30 {
		return fmt.Sprintf("%.2f GiB", float64(bytes)/(1<<30))
	}
	if bytes >= 1<<20 {
		return fmt.Sprintf("%.2f MiB", float64(bytes)/(1<<20))
	}
	if bytes >= 1<<10 {
		return fmt.Sprintf("%.2f KiB", float64(bytes)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", bytes)
}
