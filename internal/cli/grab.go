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

	var keys []string
	var keySource string
	if *apiKey != "" {
		keys = []string{*apiKey}
		keySource = "flag"
	} else {
		keys, keySource = core.ResolveAPIKeys(chosenProvider, cfg)
	}
	if len(keys) == 0 {
		fmt.Fprintf(os.Stderr, "error: no API key for provider %q\n", chosenProvider)
		fmt.Fprintf(os.Stderr, "       run `domgrab config init` or `domgrab config add-key %s <KEY>`\n", chosenProvider)
		fmt.Fprintf(os.Stderr, "       or set %s env var\n", core.ProviderEnvName(chosenProvider))
		fmt.Fprintf(os.Stderr, "       config file: %s\n", cfgPath)
		os.Exit(1)
	}

	keyring := core.NewKeyring(chosenProvider, keys)
	activeKey, activeIdx := keyring.Current()

	prov, err := buildProvider(chosenProvider, activeKey, chosenModel)
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
	if keyring.Size() > 1 {
		fmt.Printf(" provider : %s (keys: %d loaded from %s, rotating on rate limit)\n",
			prov.Name(), keyring.Size(), keySource)
		fmt.Printf(" active   : key #%d (%s)\n", activeIdx, core.MaskKey(activeKey))
	} else {
		fmt.Printf(" provider : %s (key: %s, from %s)\n", prov.Name(), core.MaskKey(activeKey), keySource)
	}
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
		_, curIdx := keyring.Current()
		if keyring.Size() > 1 {
			fmt.Printf("[batch %d] requesting %d domains (key #%d)... ", iter, *batch, curIdx)
		} else {
			fmt.Printf("[batch %d] requesting %d domains... ", iter, *batch)
		}

		callCtx, callCancel := context.WithTimeout(ctx, 120*time.Second)
		resp, err := prov.Generate(callCtx, sysPrompt, userPrompt)
		callCancel()
		if err != nil {
			// Key rotation logic: is this a rate limit error?
			if core.IsRateLimitError(err) && keyring.Size() > 1 {
				nextIdx, allExhausted, waitFor := keyring.MarkFailed(60 * time.Second)
				if allExhausted {
					fmt.Printf("ERROR: %v\n", err)
					fmt.Printf("[!] all %d keys exhausted, waiting %s for cooldown...\n",
						keyring.Size(), waitFor.Round(time.Second))
					select {
					case <-ctx.Done():
						goto done
					case <-time.After(waitFor + time.Second):
					}
					// Retry Current() after wait
					if k, i := keyring.Current(); k != "" {
						prov.SetKey(k)
						fmt.Printf("[!] resumed on key #%d\n", i)
					}
					continue
				}
				// Successfully rotated
				nextKey, _ := keyring.Current()
				prov.SetKey(nextKey)
				fmt.Printf("RATE LIMITED: %v\n", truncateErr(err, 80))
				fmt.Printf("[!] rotating to key #%d (%s available)\n", nextIdx, keyring.Status())
				continue
			}
			// Non-rotation error path (transient or fatal)
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
			filtered := make([]string, 0, len(candidates))
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
	case strings.Contains(msg, "unavailable"),
		strings.Contains(msg, "high demand"),
		strings.Contains(msg, "overloaded"):
		return 15 * time.Second
	default:
		return 2 * time.Second
	}
}

// truncateErr shortens a long error message for inline printing.
func truncateErr(err error, n int) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func buildSystemPrompt() string {
	return `Output real existing domains only, one per line.
Format: lowercase, no https://, no www., no paths, no ports.
No numbering, no markdown, no commentary, no duplicates.
Skip any domain you are uncertain exists.`
}

func buildUserPrompt(query string, batch int, tlds []string, store *core.Store, iter int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Query: %s\n", query)
	fmt.Fprintf(&sb, "Produce up to %d domains.\n", batch)

	if len(tlds) > 0 {
		fmt.Fprintf(&sb, "Only TLDs: %s\n", strings.Join(tlds, ", "))
	}

	// Token-efficient dedup hints:
	// - When store is empty/small, skip AVOID list entirely (no duplicates possible)
	// - When store is medium, send a small sample (40 domains)
	// - When store is large, send top TLD histogram + small sample
	size := store.Size()
	if size > 0 && size < 50 {
		// Small store: no sample needed, AI likely won't repeat naturally
		fmt.Fprintf(&sb, "\nWe already have %d domains. Generate new ones.\n", size)
	} else if size >= 50 {
		// Medium+ store: send small random sample to prime the AI
		sample := store.Sample(40)
		if len(sample) > 0 {
			sb.WriteString("\nDo NOT repeat these (or similar):\n")
			for _, d := range sample {
				sb.WriteString(d)
				sb.WriteString("\n")
			}
		}
	}

	if iter > 1 {
		fmt.Fprintf(&sb, "\nBatch #%d — explore different angles from prior batches.\n", iter)
	}

	sb.WriteString("\nOutput: domain list only.")
	return sb.String()
}