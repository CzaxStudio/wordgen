package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ─── ANSI Colors ────────────────────────────────────────────────────────────

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
)

func banner() {
	fmt.Println(Cyan + Bold + `
 ██╗    ██╗ ██████╗ ██████╗ ██████╗  ██████╗ ███████╗███╗   ██╗
 ██║    ██║██╔═══██╗██╔══██╗██╔══██╗██╔════╝ ██╔════╝████╗  ██║
 ██║ █╗ ██║██║   ██║██████╔╝██║  ██║██║  ███╗█████╗  ██╔██╗ ██║
 ██║███╗██║██║   ██║██╔══██╗██║  ██║██║   ██║██╔══╝  ██║╚██╗██║
 ╚███╔███╔╝╚██████╔╝██║  ██║██████╔╝╚██████╔╝███████╗██║ ╚████║
  ╚══╝╚══╝  ╚═════╝ ╚═╝  ╚═╝╚═════╝  ╚═════╝ ╚══════╝╚═╝  ╚═══╝
` + Reset)
	fmt.Println(Dim + "  Professional Wordlist Generator — For Authorized Pen Testing Only" + Reset)
	fmt.Println(Dim + "  ─────────────────────────────────────────────────────────────────" + Reset)
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
}

// ─── Charset Presets ─────────────────────────────────────────────────────────

var charsets = map[string]string{
	"lower":   "abcdefghijklmnopqrstuvwxyz",
	"upper":   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
	"digits":  "0123456789",
	"special": "!@#$%^&*()_+-=[]{}|;':\",./<>?",
	"alpha":   "abcdefghijklmnopqrstuvwxyzABCCDEFGHIJKLMNOPQRSTUVWXYZ",
	"alnum":   "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
	"all":     "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*",
}

// ─── Leet Speak Map ──────────────────────────────────────────────────────────

var leetMap = map[rune][]string{
	'a': {"4", "@"},
	'e': {"3"},
	'i': {"1", "!"},
	'o': {"0"},
	's': {"5", "$"},
	't': {"7"},
	'l': {"1"},
	'g': {"9"},
}

// ─── Mutation Engine ─────────────────────────────────────────────────────────

func mutate(word string, cfg *Config) []string {
	variants := map[string]bool{word: true}

	// Capitalize variants
	if cfg.Capitalize {
		variants[strings.ToUpper(word)] = true
		variants[strings.ToLower(word)] = true
		variants[strings.Title(strings.ToLower(word))] = true
		// CamelCase first letter
		if len(word) > 0 {
			cap := strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
			variants[cap] = true
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

	// Year appends
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
		specials := []string{"!", "@", "#", "$", "*", "123", "1", "12"}
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
		fmt.Printf(Yellow+"  [~] Brute force: charset=%s, len=%d-%d, ~%s combos\n"+Reset,
			cfg.Charset, cfg.MinLen, cfg.MaxLen, formatNum(total))
	}

	for length := cfg.MinLen; length <= cfg.MaxLen; length++ {
		generateCombinations(chars, length, writer, counter, mu)
	}
}

func generateCombinations(chars []rune, length int, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	indices := make([]int, length)
	n := len(chars)

	for {
		var sb strings.Builder
		for _, idx := range indices {
			sb.WriteRune(chars[idx])
		}
		mu.Lock()
		writer.WriteString(sb.String() + "\n")
		*counter++
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

// ─── Mode: Keyword Mutation ──────────────────────────────────────────────────

func keywordMode(cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	keywords := cfg.Keywords

	// Load from file if provided
	if cfg.KeyFile != "" {
		f, err := os.Open(cfg.KeyFile)
		if err != nil {
			fmt.Println(Red + "  [!] Cannot open keyword file: " + err.Error() + Reset)
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

	if len(keywords) == 0 {
		fmt.Println(Red + "  [!] No keywords provided. Use -keywords or -keyfile." + Reset)
		return
	}

	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Keyword mutation: %d base words\n"+Reset, len(keywords))
	}

	seen := map[string]bool{}
	for _, kw := range keywords {
		mutations := mutate(kw, cfg)
		for _, m := range mutations {
			if !seen[m] {
				seen[m] = true
				mu.Lock()
				writer.WriteString(m + "\n")
				*counter++
				mu.Unlock()
			}
		}
	}
}

// ─── Mode: Combinator ────────────────────────────────────────────────────────

func combinatorMode(cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	words := cfg.Keywords
	if cfg.KeyFile != "" {
		f, err := os.Open(cfg.KeyFile)
		if err != nil {
			fmt.Println(Red + "  [!] Cannot open keyword file: " + err.Error() + Reset)
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" && isAlphanumeric(line) {
				words = append(words, line)
			}
		}
	}

	seps := []string{"", "_", "-", ".", "123", "!"}

	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Combinator: %d words × %d words × %d separators\n"+Reset,
			len(words), len(words), len(seps))
	}

	for _, w1 := range words {
		for _, w2 := range words {
			for _, sep := range seps {
				combo := w1 + sep + w2
				if len(combo) >= cfg.MinLen && (cfg.MaxLen == 0 || len(combo) <= cfg.MaxLen) {
					mu.Lock()
					writer.WriteString(combo + "\n")
					*counter++
					mu.Unlock()
				}
			}
		}
	}
}

// ─── Mode: Pattern ───────────────────────────────────────────────────────────

// Pattern syntax: ? = letter, # = digit, @ = special, * = any alnum
func patternMode(pattern string, cfg *Config, writer *bufio.Writer, counter *int64, mu *sync.Mutex) {
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	digits := []rune("0123456789")
	specials := []rune("!@#$%")
	alnum := []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	var expand func(pos int, current string)
	expand = func(pos int, current string) {
		if pos == len(pattern) {
			mu.Lock()
			writer.WriteString(current + "\n")
			*counter++
			mu.Unlock()
			return
		}

		ch := rune(pattern[pos])
		switch ch {
		case '?':
			for _, l := range letters {
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
		default:
			expand(pos+1, current+string(ch))
		}
	}

	if !cfg.Quiet {
		fmt.Printf(Yellow+"  [~] Pattern: \"%s\"\n"+Reset, pattern)
	}
	expand(0, "")
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
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	} else if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	} else if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

// ─── Progress Ticker ─────────────────────────────────────────────────────────

func startProgress(counter *int64, done chan bool, quiet bool) {
	if quiet {
		return
	}
	go func() {
		start := time.Now()
		for {
			select {
			case <-done:
				return
			case <-time.After(1 * time.Second):
				elapsed := time.Since(start).Seconds()
				rate := float64(*counter) / elapsed
				fmt.Printf("\r"+Cyan+"  [+] Words: %-10s  Rate: %-10s  Elapsed: %.0fs"+Reset,
					formatNum(*counter), formatNum(int64(rate))+"/s", elapsed)
			}
		}
	}()
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	banner()

	// Flags
	mode := flag.String("mode", "keyword", "Mode: brute | keyword | combinator | pattern")
	output := flag.String("output", "wordlist.txt", "Output file path")
	minLen := flag.Int("min", 4, "Minimum word length")
	maxLen := flag.Int("max", 12, "Maximum word length")
	charset := flag.String("charset", "alnum", "Charset for brute mode: lower|upper|digits|alpha|alnum|all|<custom>")
	keywords := flag.String("keywords", "", "Comma-separated keywords (keyword/combinator mode)")
	keyFile := flag.String("keyfile", "", "File with one keyword per line")
	leet := flag.Bool("leet", false, "Apply leet speak substitutions")
	capitalize := flag.Bool("cap", true, "Apply capitalization variants")
	appendStr := flag.String("append", "", "Append string to every word")
	prependStr := flag.String("prepend", "", "Prepend string to every word")
	years := flag.Bool("years", false, "Append common years (1990-now)")
	special := flag.Bool("special", false, "Append common special chars/numbers")
	pattern := flag.String("pattern", "", "Pattern for pattern mode: ? letter, # digit, @ special")
	quiet := flag.Bool("quiet", false, "Suppress progress output")

	flag.Usage = func() {
		fmt.Println(Bold + "  Usage:" + Reset)
		fmt.Println("    wordgen -mode=<mode> [options]")
		fmt.Println()
		fmt.Println(Bold + "  Modes:" + Reset)
		fmt.Println("    brute       Brute-force all combinations")
		fmt.Println("    keyword     Mutate keywords (leet, caps, years, specials)")
		fmt.Println("    combinator  Combine two word lists")
		fmt.Println("    pattern     Generate from pattern (e.g. -pattern=admin##)")
		fmt.Println()
		fmt.Println(Bold + "  Examples:" + Reset)
		fmt.Println("    wordgen -mode=keyword -keywords=john,admin -leet -years -cap")
		fmt.Println("    wordgen -mode=brute -charset=alnum -min=4 -max=6")
		fmt.Println("    wordgen -mode=combinator -keyfile=names.txt -append=123")
		fmt.Println("    wordgen -mode=pattern -pattern=admin##")
		fmt.Println()
		flag.PrintDefaults()
	}

	flag.Parse()

	cfg := &Config{
		Mode:       *mode,
		Output:     *output,
		MinLen:     *minLen,
		MaxLen:     *maxLen,
		Charset:    *charset,
		KeyFile:    *keyFile,
		Leet:       *leet,
		Capitalize: *capitalize,
		Append:     *appendStr,
		Prepend:    *prependStr,
		Years:      *years,
		Special:    *special,
		Quiet:      *quiet,
	}

	if *keywords != "" {
		for _, kw := range strings.Split(*keywords, ",") {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				cfg.Keywords = append(cfg.Keywords, kw)
			}
		}
	}

	// Ensure output dir exists
	dir := filepath.Dir(cfg.Output)
	if dir != "." {
		os.MkdirAll(dir, 0755)
	}

	// Open output file
	f, err := os.Create(cfg.Output)
	if err != nil {
		fmt.Println(Red + "  [!] Cannot create output file: " + err.Error() + Reset)
		os.Exit(1)
	}
	defer f.Close()

	writer := bufio.NewWriterSize(f, 1<<20) // 1MB buffer
	var counter int64
	var mu sync.Mutex
	done := make(chan bool)

	fmt.Printf(Green+"  [*] Mode      : %s\n"+Reset, strings.ToUpper(cfg.Mode))
	fmt.Printf(Green+"  [*] Output    : %s\n"+Reset, cfg.Output)
	fmt.Printf(Green+"  [*] Length    : %d - %d\n"+Reset, cfg.MinLen, cfg.MaxLen)
	if cfg.Leet {
		fmt.Println(Green + "  [*] Leet      : enabled" + Reset)
	}
	if cfg.Years {
		fmt.Println(Green + "  [*] Years     : enabled" + Reset)
	}
	fmt.Println()

	start := time.Now()
	startProgress(&counter, done, cfg.Quiet)

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
	default:
		fmt.Println(Red + "  [!] Unknown mode: " + cfg.Mode + Reset)
		flag.Usage()
		os.Exit(1)
	}

	writer.Flush()
	done <- true

	elapsed := time.Since(start)
	info, _ := f.Stat()
	size := ""
	if info != nil {
		mb := float64(info.Size()) / 1024 / 1024
		size = fmt.Sprintf("%.2f MB", mb)
	}

	fmt.Printf("\n\n" + Green + Bold + "  ✓ Done!" + Reset + "\n")
	fmt.Printf(Green+"  [+] Total words : %s\n"+Reset, formatNum(counter))
	fmt.Printf(Green+"  [+] File size   : %s\n"+Reset, size)
	fmt.Printf(Green+"  [+] Time taken  : %s\n"+Reset, elapsed.Round(time.Millisecond))
	fmt.Printf(Green+"  [+] Output file : %s\n\n"+Reset, cfg.Output)
}
