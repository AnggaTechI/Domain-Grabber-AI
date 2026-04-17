package cli

import (
	"bufio"
	"domgrab/internal/core"
	"fmt"
	"os"
	"strings"
)

func runConfig(args []string) {
	if len(args) == 0 {
		printConfigUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "show":
		runConfigShow()
	case "path":
		fmt.Println(core.ConfigPath())
	case "set":
		runConfigSet(args[1:])
	case "get":
		runConfigGet(args[1:])
	case "init":
		runConfigInit()
	case "unset":
		runConfigUnset(args[1:])
	default:
		printConfigUsage()
		os.Exit(1)
	}
}

func printConfigUsage() {
	fmt.Println(`domgrab config - manage the configuration file

USAGE:
    domgrab config <subcommand> [args]

SUBCOMMANDS:
    init                       Interactive setup
    show                       Print current config (API keys are masked)
    path                       Print config file location
    set <key> <value>          Set a single config field
    get <key>                  Print a single config field (unmasked)
    unset <key>                Remove a config field

KEYS:
    anthropic_api_key
    openai_api_key
    gemini_api_key
    groq_api_key
    openrouter_api_key
    default_provider
    default_model
    default_output
    anthropic_model
    openai_model
    gemini_model
    groq_model
    openrouter_model

EXAMPLES:
    domgrab config init
    domgrab config set gemini_api_key YOUR_KEY
    domgrab config set default_provider gemini
    domgrab config set gemini_model gemini-3-flash-preview
    domgrab config show
    domgrab config unset openai_api_key`)
}

func runConfigShow() {
	cfg, path, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Config file: %s\n", path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("(file does not exist yet - run `domgrab config init`)")
		return
	}
	fmt.Println()
	fmt.Printf("  anthropic_api_key  : %s\n", displayKey(cfg.AnthropicAPIKey))
	fmt.Printf("  openai_api_key     : %s\n", displayKey(cfg.OpenAIAPIKey))
	fmt.Printf("  gemini_api_key     : %s\n", displayKey(cfg.GeminiAPIKey))
	fmt.Printf("  groq_api_key       : %s\n", displayKey(cfg.GroqAPIKey))
	fmt.Printf("  openrouter_api_key : %s\n", displayKey(cfg.OpenRouterAPIKey))
	fmt.Printf("  default_provider   : %s\n", displayStr(cfg.DefaultProvider))
	fmt.Printf("  default_model      : %s\n", displayStr(cfg.DefaultModel))
	fmt.Printf("  default_output     : %s\n", displayStr(cfg.DefaultOutput))
	fmt.Printf("  anthropic_model    : %s\n", displayStr(cfg.AnthropicModel))
	fmt.Printf("  openai_model       : %s\n", displayStr(cfg.OpenAIModel))
	fmt.Printf("  gemini_model       : %s\n", displayStr(cfg.GeminiModel))
	fmt.Printf("  groq_model         : %s\n", displayStr(cfg.GroqModel))
	fmt.Printf("  openrouter_model   : %s\n", displayStr(cfg.OpenRouterModel))
}

func displayKey(k string) string {
	if k == "" {
		return "(not set)"
	}
	return core.MaskKey(k)
}

func displayStr(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func runConfigSet(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: domgrab config set <key> <value>")
		os.Exit(1)
	}
	key := args[0]
	value := strings.Join(args[1:], " ")

	cfg, _, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := setConfigField(cfg, key, value); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	path, err := core.SaveConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("saved %s to %s\n", key, path)
}

func runConfigGet(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: domgrab config get <key>")
		os.Exit(1)
	}
	cfg, _, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	v, err := getConfigField(cfg, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(v)
}

func runConfigUnset(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: domgrab config unset <key>")
		os.Exit(1)
	}
	cfg, _, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := setConfigField(cfg, args[0], ""); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	path, err := core.SaveConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("unset %s in %s\n", args[0], path)
}

func runConfigInit() {
	cfg, path, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Setting up domgrab config at:\n  %s\n\n", path)
	fmt.Println("Press Enter to skip a field (keep existing value).")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	prompt := func(label, current string, mask bool) string {
		shown := "(empty)"
		if current != "" {
			if mask {
				shown = core.MaskKey(current)
			} else {
				shown = current
			}
		}
		fmt.Printf("%s [%s]: ", label, shown)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	if v := prompt("Anthropic API key", cfg.AnthropicAPIKey, true); v != "" { cfg.AnthropicAPIKey = v }
	if v := prompt("OpenAI API key", cfg.OpenAIAPIKey, true); v != "" { cfg.OpenAIAPIKey = v }
	if v := prompt("Gemini API key", cfg.GeminiAPIKey, true); v != "" { cfg.GeminiAPIKey = v }
	if v := prompt("Groq API key", cfg.GroqAPIKey, true); v != "" { cfg.GroqAPIKey = v }
	if v := prompt("OpenRouter API key", cfg.OpenRouterAPIKey, true); v != "" { cfg.OpenRouterAPIKey = v }

	if v := prompt("Default provider (anthropic|openai|gemini|groq|openrouter)", cfg.DefaultProvider, false); v != "" {
		if !core.ValidProvider(v) {
			fmt.Fprintf(os.Stderr, "warning: unknown provider %q, ignoring\n", v)
		} else {
			cfg.DefaultProvider = strings.ToLower(strings.TrimSpace(v))
		}
	}
	if v := prompt("Default model (legacy fallback, blank = auto)", cfg.DefaultModel, false); v != "" { cfg.DefaultModel = v }
	if v := prompt("Default output file", cfg.DefaultOutput, false); v != "" { cfg.DefaultOutput = v }
	if v := prompt("Anthropic model", cfg.AnthropicModel, false); v != "" { cfg.AnthropicModel = v }
	if v := prompt("OpenAI model", cfg.OpenAIModel, false); v != "" { cfg.OpenAIModel = v }
	if v := prompt("Gemini model", cfg.GeminiModel, false); v != "" { cfg.GeminiModel = v }
	if v := prompt("Groq model", cfg.GroqModel, false); v != "" { cfg.GroqModel = v }
	if v := prompt("OpenRouter model", cfg.OpenRouterModel, false); v != "" { cfg.OpenRouterModel = v }

	savedPath, err := core.SaveConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nconfig saved to %s (mode 0600)\n", savedPath)
}

func setConfigField(cfg *core.Config, key, value string) error {
	switch key {
	case "anthropic_api_key":
		cfg.AnthropicAPIKey = value
	case "openai_api_key":
		cfg.OpenAIAPIKey = value
	case "gemini_api_key":
		cfg.GeminiAPIKey = value
	case "groq_api_key":
		cfg.GroqAPIKey = value
	case "openrouter_api_key":
		cfg.OpenRouterAPIKey = value
	case "default_provider":
		if value != "" && !core.ValidProvider(value) {
			return fmt.Errorf("default_provider must be one of: anthropic, openai, gemini, groq, openrouter")
		}
		cfg.DefaultProvider = strings.ToLower(strings.TrimSpace(value))
	case "default_model":
		cfg.DefaultModel = value
	case "default_output":
		cfg.DefaultOutput = value
	case "anthropic_model":
		cfg.AnthropicModel = value
	case "openai_model":
		cfg.OpenAIModel = value
	case "gemini_model":
		cfg.GeminiModel = value
	case "groq_model":
		cfg.GroqModel = value
	case "openrouter_model":
		cfg.OpenRouterModel = value
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func getConfigField(cfg *core.Config, key string) (string, error) {
	switch key {
	case "anthropic_api_key":
		return cfg.AnthropicAPIKey, nil
	case "openai_api_key":
		return cfg.OpenAIAPIKey, nil
	case "gemini_api_key":
		return cfg.GeminiAPIKey, nil
	case "groq_api_key":
		return cfg.GroqAPIKey, nil
	case "openrouter_api_key":
		return cfg.OpenRouterAPIKey, nil
	case "default_provider":
		return cfg.DefaultProvider, nil
	case "default_model":
		return cfg.DefaultModel, nil
	case "default_output":
		return cfg.DefaultOutput, nil
	case "anthropic_model":
		return cfg.AnthropicModel, nil
	case "openai_model":
		return cfg.OpenAIModel, nil
	case "gemini_model":
		return cfg.GeminiModel, nil
	case "groq_model":
		return cfg.GroqModel, nil
	case "openrouter_model":
		return cfg.OpenRouterModel, nil
	default:
		return "", fmt.Errorf("unknown config key %q", key)
	}
}