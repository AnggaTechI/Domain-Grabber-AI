package cli

import (
	"domgrab/internal/core"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	Version = "1.0.0"
	Author  = "AnggaTechI"
	GitHub  = "https://github.com/AnggaTechI"
)

func Main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "grab":
		runGrab(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "stats":
		runStats(os.Args[2:])
	case "config":
		runConfig(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("domgrab v%s\n", Version)
		fmt.Printf("Author : %s\n", Author)
		fmt.Printf("GitHub : %s\n", GitHub)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`domgrab - AI-powered domain grabber (CLI)

AUTHOR:
    AnggaTechI
    https://github.com/AnggaTechI

USAGE:
    domgrab <command> [flags]

COMMANDS:
    grab      Grab domains via AI based on a natural language query
    list      Show domains in the master list (optionally filtered)
    stats     Show master list statistics
    config    Manage API keys & defaults (stored in JSON)
    version   Print version
    help      Show this help

GRAB FLAGS:
    --provider     AI provider: anthropic | openai | gemini | groq | openrouter
                   (default: from config, else first available key, else anthropic)
    --query        Natural language query (required)
                   e.g. "government domains from Brazil"
    --target       Target number of new domains to collect (default: 500)
    --batch        Domains to request per API call (default: 50)
    --output       Master list file (default: master.txt)
    --model        Override default model
    --api-key      Override API key for this run
    --tld          Filter: only keep domains matching these TLDs (comma-separated)
                   e.g. --tld "gov.br,edu.br"
    --dry-run      Print what would be added without writing to file
    --verbose      Show raw AI responses

LIST FLAGS:
    --output       Master list file (default: master.txt)
    --filter       Substring filter (case-insensitive)
    --tld          TLD filter (comma-separated)
    --limit        Max rows to print (default: all)

STATS FLAGS:
    --output       Master list file (default: master.txt)

API KEYS:
    Stored in JSON config file (run ` + "`" + `domgrab config init` + "`" + ` to set up).
    Config is portable: ./domgrab.json in current working directory.
    Resolution order: --api-key flag > env var > config file.
    Env vars: ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY,
              GROQ_API_KEY, OPENROUTER_API_KEY

MODEL KEYS IN JSON:
    anthropic_model
    openai_model
    gemini_model
    groq_model
    openrouter_model

EXAMPLES:
    domgrab config init
    domgrab config set gemini_api_key YOUR_KEY
    domgrab config set gemini_model gemini-3-flash-preview
    domgrab config set default_provider gemini
    domgrab grab --query "universitas di Indonesia" --target 100 --batch 20 --tld ac.id
    domgrab list --tld ac.id --limit 50
    domgrab stats`)
}

func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "run `domgrab help` for full usage\n")
	}
	return fs
}

// resolveOutput returns the user's --output flag if set, else config default, else master.txt.
func resolveOutput(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	cfg, _, err := core.LoadConfig()
	if err == nil && cfg.DefaultOutput != "" {
		return cfg.DefaultOutput
	}
	return "master.txt"
}

func runList(args []string) {
	fs := flagSet("list")
	outPath := fs.String("output", "", "Master list file")
	filter := fs.String("filter", "", "Substring filter")
	tldFlag := fs.String("tld", "", "TLD filter (comma-separated)")
	limit := fs.Int("limit", 0, "Max rows (0 = all)")
	_ = fs.Parse(args)

	path := resolveOutput(*outPath)
	store, err := core.NewStore(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	tlds := parseCSV(*tldFlag)
	results := store.Filter(*filter, tlds)
	n := len(results)
	if *limit > 0 && *limit < n {
		results = results[:*limit]
	}
	for _, d := range results {
		fmt.Println(d)
	}
	fmt.Fprintf(os.Stderr, "\n(%d shown / %d matched / %d total)\n", len(results), n, store.Size())
}

func runStats(args []string) {
	fs := flagSet("stats")
	outPath := fs.String("output", "", "Master list file")
	_ = fs.Parse(args)

	path := resolveOutput(*outPath)
	store, err := core.NewStore(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	total := store.Size()
	fmt.Printf("Master list : %s\n", path)
	fmt.Printf("Total       : %d domains\n\n", total)
	if total == 0 {
		return
	}

	h := store.TLDHistogram()
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(h))
	for k, v := range h {
		pairs = append(pairs, kv{k: k, v: v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})

	fmt.Println("Top TLDs:")
	show := 30
	if show > len(pairs) {
		show = len(pairs)
	}
	for i := 0; i < show; i++ {
		pct := float64(pairs[i].v) * 100 / float64(total)
		fmt.Printf("  .%-10s %7d  (%.1f%%)\n", pairs[i].k, pairs[i].v, pct)
	}
	if len(pairs) > show {
		fmt.Printf("  ... and %d more TLDs\n", len(pairs)-show)
	}
}