package cli

import (
	"context"
	"domgrab/internal/core"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func runGrab(args []string) {
	fs := flagSet("grab")
	provider := fs.String("provider", "", "AI provider: anthropic|openai|gemini|groq|openrouter")
	query := fs.String("query", "", "Natural language query (required)")
	target := fs.Int("target", 500, "Target new domains to collect")
	batch := fs.Int("batch", 50, "Domains per API call")
	outPath := fs.String("output", "", "Master list file (default from config, else master.txt)")
	model := fs.String("model", "", "Override default model")
	apiKey := fs.String("api-key", "", "Override API key (takes precedence over env & config)")
	tldFlag := fs.String("tld", "", "TLD filter (comma-separated)")
	dryRun := fs.Bool("dry-run", false, "Don't write to file")
	verbose := fs.Bool("verbose", false, "Print raw AI responses")
	_ = fs.Parse(args)

	if *query == "" {
		fmt.Fprintln(os.Stderr, "error: --query is required")
		os.Exit(1)
	}
	if *target <= 0 || *batch <= 0 {
		fmt.Fprintln(os.Stderr, "error: --target and --batch must be positive")
		os.Exit(1)
	}

	cfg, cfgPath, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not read config (%v), continuing\n", err)
		cfg = &core.Config{}
	}

	chosenProvider := core.ResolveProvider(*provider, cfg)
	chosenOutput := *outPath
	if chosenOutput == "" {
		chosenOutput = cfg.DefaultOutput
	}
	if chosenOutput == "" {
		chosenOutput = "master.txt"
	}
	chosenModel := core.ResolveModel(chosenProvider, *model, cfg)
	tlds := parseCSV(*tldFlag)

	var key, keySource string
	if *apiKey != "" {
		key = *apiKey
		keySource = "flag"
	} else {
		key, keySource = core.ResolveAPIKey(chosenProvider, cfg)
	}
	if key == "" {
		fmt.Fprintf(os.Stderr, "error: no API key for provider %q\n", chosenProvider)
		fmt.Fprintf(os.Stderr, "       run `domgrab config init` or set %s env var\n", core.ProviderEnvName(chosenProvider))
		fmt.Fprintf(os.Stderr, "       config file: %s\n", cfgPath)
		os.Exit(1)
	}

	prov, err := buildProvider(chosenProvider, key, chosenModel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	store, err := core.NewStore(chosenOutput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading %s: %v\n", chosenOutput, err)
		os.Exit(1)
	}

	fmt.Printf("═══════════════════════════════════════════\n")
	fmt.Printf(" domgrab v%s\n", Version)
	fmt.Printf(" author   : %s\n", Author)
	fmt.Printf(" github   : %s\n", GitHub)
	fmt.Printf("═══════════════════════════════════════════\n")
	fmt.Printf(" provider : %s (key: %s, from %s)\n", prov.Name(), core.MaskKey(key), keySource)
	if chosenModel != "" {
		fmt.Printf(" model    : %s\n", chosenModel)
	}
	fmt.Printf(" query    : %s\n", *query)
	fmt.Printf(" target   : %d new domains\n", *target)
	fmt.Printf(" batch    : %d per request\n", *batch)
	fmt.Printf(" output   : %s (currently %d domains)\n", chosenOutput, store.Size())
	if len(tlds) > 0 {
		fmt.Printf(" tld      : %s\n", strings.Join(tlds, ", "))
	}
	if *dryRun {
		fmt.Printf(" MODE     : DRY RUN (no file writes)\n")
	}
	fmt.Printf("═══════════════════════════════════════════\n\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n[!] Interrupt received, finishing current batch then stopping...")
		cancel()
	}()

	sysPrompt := buildSystemPrompt()
	added := 0
	iter := 0
	consecutiveEmpty := 0
	startTime := time.Now()

	for added < *target {
		iter++
		if ctx.Err() != nil {
			break
		}

		userPrompt := buildUserPrompt(*query, *batch, tlds, store, iter)
		fmt.Printf("[batch %d] requesting %d domains... ", iter, *batch)

		callCtx, callCancel := context.WithTimeout(ctx, 120*time.Second)
		resp, err := prov.Generate(callCtx, sysPrompt, userPrompt)
		callCancel()
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			wait := waitDuration(err)
			select {
			case <-ctx.Done():
				goto done
			case <-time.After(wait):
			}
			continue
		}

		if *verbose {
			fmt.Printf("\n--- raw response ---\n%s\n--------------------\n", resp)
		}

		candidates := core.ExtractDomains(resp)
		if len(tlds) > 0 {
			filtered := candidates[:0]
			for _, d := range candidates {
				if core.MatchesAnyTLD(d, tlds) {
					filtered = append(filtered, d)
				}
			}
			candidates = filtered
		}

		var newOnes []string
		for _, d := range candidates {
			if !store.Has(d) {
				newOnes = append(newOnes, d)
			}
		}

		actuallyAdded := store.AddMany(newOnes)
		if !*dryRun && len(actuallyAdded) > 0 {
			if err := store.Append(actuallyAdded); err != nil {
				fmt.Printf("\n[!] write error: %v\n", err)
			}
		}

		added += len(actuallyAdded)
		fmt.Printf("got %d, %d new (total: %d/%d)\n", len(candidates), len(actuallyAdded), added, *target)

		if len(actuallyAdded) == 0 {
			consecutiveEmpty++
			if consecutiveEmpty >= 3 {
				fmt.Println("\n[!] 3 consecutive batches with 0 new domains — AI likely exhausted. Stopping.")
				break
			}
		} else {
			consecutiveEmpty = 0
		}

		select {
		case <-ctx.Done():
			goto done
		case <-time.After(500 * time.Millisecond):
		}
	}

done:
	elapsed := time.Since(startTime).Round(time.Second)
	fmt.Printf("\n═══════════════════════════════════════════\n")
	fmt.Printf(" done in %s\n", elapsed)
	fmt.Printf(" added %d new domains\n", added)
	fmt.Printf(" master list now: %d domains\n", store.Size())
	fmt.Printf("═══════════════════════════════════════════\n")
}

func buildProvider(provider, key, model string) (core.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		return core.NewAnthropicProvider(key, model), nil
	case "openai":
		return core.NewOpenAIProvider(key, model), nil
	case "gemini":
		return core.NewGeminiProvider(key, model), nil
	case "groq":
		return core.NewGroqProvider(key, model), nil
	case "openrouter":
		return core.NewOpenRouterProvider(key, model), nil
	default:
		return nil, fmt.Errorf("unknown provider %q (valid: anthropic, openai, gemini, groq, openrouter)", provider)
	}
}

func waitDuration(err error) time.Duration {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "resource_exhausted"), strings.Contains(msg, "quota"), strings.Contains(msg, "rate limit"):
		return 5 * time.Second
	default:
		return 2 * time.Second
	}
}

func buildSystemPrompt() string {
	return `You are a domain research assistant. Your job is to produce lists of real, existing internet domains that match the user's query.

CRITICAL RULES:
1. Output ONLY domain names, one per line. No numbering, no descriptions, no markdown, no commentary.
2. Output only registered, real-world domains — no hypothetical or invented ones.
3. Use canonical form: lowercase, no "https://", no "www.", no trailing slash, no paths.
4. If the user provides a list of domains to AVOID, do NOT include any of them in your output.
5. Do not include IP addresses, email addresses, or URLs with paths.
6. If you are uncertain whether a domain exists, omit it.
7. Prefer diversity — do not return near-duplicates (e.g. both "example.gov.br" and "www.example.gov.br").`
}

func buildUserPrompt(query string, batch int, tlds []string, store *core.Store, iter int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Query: %s\n\n", query)
	fmt.Fprintf(&sb, "Produce up to %d domains matching this query.\n", batch)

	if len(tlds) > 0 {
		fmt.Fprintf(&sb, "Only include domains ending in: %s\n", strings.Join(tlds, ", "))
	}

	sample := store.Sample(150)
	if len(sample) > 0 {
		sb.WriteString("\nAVOID these domains (already in our list). Also avoid anything that looks similar:\n")
		for _, d := range sample {
			sb.WriteString(d)
			sb.WriteString("\n")
		}
	}

	if iter > 1 {
		fmt.Fprintf(&sb, "\nThis is continuation batch #%d. Focus on domains you haven't yet suggested in earlier batches of this session. Explore less obvious corners of the query space.\n", iter)
	}

	sb.WriteString("\nOutput: domain list only, one per line.")
	return sb.String()
}