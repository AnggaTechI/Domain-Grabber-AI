package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the JSON structure stored at ./domgrab.json in the CWD.
type Config struct {
	// Legacy single-key fields (backward compatible - auto-merged into *APIKeys)
	AnthropicAPIKey  string `json:"anthropic_api_key"`
	OpenAIAPIKey     string `json:"openai_api_key"`
	GeminiAPIKey     string `json:"gemini_api_key"`
	GroqAPIKey       string `json:"groq_api_key"`
	OpenRouterAPIKey string `json:"openrouter_api_key"`

	// Multi-key arrays for rotation on rate limit
	AnthropicAPIKeys  []string `json:"anthropic_api_keys,omitempty"`
	OpenAIAPIKeys     []string `json:"openai_api_keys,omitempty"`
	GeminiAPIKeys     []string `json:"gemini_api_keys,omitempty"`
	GroqAPIKeys       []string `json:"groq_api_keys,omitempty"`
	OpenRouterAPIKeys []string `json:"openrouter_api_keys,omitempty"`

	DefaultProvider string `json:"default_provider"`
	DefaultModel    string `json:"default_model"` // legacy fallback
	DefaultOutput   string `json:"default_output"`

	AnthropicModel  string `json:"anthropic_model"`
	OpenAIModel     string `json:"openai_model"`
	GeminiModel     string `json:"gemini_model"`
	GroqModel       string `json:"groq_model"`
	OpenRouterModel string `json:"openrouter_model"`
}

// configPath returns the path to the config file.
// Config is always stored as ./domgrab.json in the current working directory
// (portable mode). This lets users keep separate configs per project folder.
// Override with $DOMGRAB_CONFIG env var if needed.
func configPath() string {
	if p := os.Getenv("DOMGRAB_CONFIG"); p != "" {
		return p
	}
	abs, err := filepath.Abs("domgrab.json")
	if err != nil {
		return "domgrab.json"
	}
	return abs
}

// ConfigPath exposes the resolved config path to other packages.
func ConfigPath() string {
	return configPath()
}

// LoadConfig reads the config file. Returns empty Config (not error) if file
// doesn't exist — that's a valid state (user may supply keys via env/flag).
func LoadConfig() (*Config, string, error) {
	path := configPath()
	cfg := &Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, path, nil
		}
		return nil, path, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, path, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	return cfg, path, nil
}

// SaveConfig writes the config to disk with tight permissions (0600).
func SaveConfig(cfg *Config) (string, error) {
	path := configPath()
	if envPath := os.Getenv("DOMGRAB_CONFIG"); envPath != "" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return path, err
		}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return path, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return path, err
	}
	return path, nil
}

// ResolveAPIKey determines which API key to use given precedence:
//  1. Environment variable
//  2. Config file (single key)
//  3. First key in the multi-key array
// Returns the key and a short string indicating the source (for log messages).
func ResolveAPIKey(provider string, cfg *Config) (key string, source string) {
	keys, source := ResolveAPIKeys(provider, cfg)
	if len(keys) == 0 {
		return "", ""
	}
	return keys[0], source
}

// ResolveAPIKeys returns ALL available keys for a provider, merged from:
//  1. Environment variable (if set, as single key)
//  2. Single-key config field (legacy)
//  3. Multi-key array field
// Deduplicates and preserves order. Used for key rotation.
func ResolveAPIKeys(provider string, cfg *Config) (keys []string, source string) {
	seen := make(map[string]struct{})
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k == "" {
			return
		}
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}

	var envKey, envName string
	var single string
	var arr []string

	switch normalizeProvider(provider) {
	case "anthropic":
		envName = "ANTHROPIC_API_KEY"
		envKey = os.Getenv(envName)
		if cfg != nil {
			single = cfg.AnthropicAPIKey
			arr = cfg.AnthropicAPIKeys
		}
	case "openai":
		envName = "OPENAI_API_KEY"
		envKey = os.Getenv(envName)
		if cfg != nil {
			single = cfg.OpenAIAPIKey
			arr = cfg.OpenAIAPIKeys
		}
	case "gemini":
		envName = "GEMINI_API_KEY"
		envKey = os.Getenv(envName)
		if envKey == "" {
			envKey = os.Getenv("GOOGLE_API_KEY")
			if envKey != "" {
				envName = "GOOGLE_API_KEY"
			}
		}
		if cfg != nil {
			single = cfg.GeminiAPIKey
			arr = cfg.GeminiAPIKeys
		}
	case "groq":
		envName = "GROQ_API_KEY"
		envKey = os.Getenv(envName)
		if cfg != nil {
			single = cfg.GroqAPIKey
			arr = cfg.GroqAPIKeys
		}
	case "openrouter":
		envName = "OPENROUTER_API_KEY"
		envKey = os.Getenv(envName)
		if cfg != nil {
			single = cfg.OpenRouterAPIKey
			arr = cfg.OpenRouterAPIKeys
		}
	default:
		return nil, ""
	}

	// Determine source label based on where the first key came from
	if envKey != "" {
		add(envKey)
		source = "env:" + envName
	}
	if single != "" {
		add(single)
		if source == "" {
			source = "config"
		}
	}
	for _, k := range arr {
		add(k)
	}
	if source == "" && len(keys) > 0 {
		source = "config"
	}
	return keys, source
}

// ResolveModel chooses a model with precedence:
//  1. --model flag
//  2. provider-specific model in config
//  3. legacy default_model in config
//  4. provider constructor default
func ResolveModel(provider, flagModel string, cfg *Config) string {
	if strings.TrimSpace(flagModel) != "" {
		return strings.TrimSpace(flagModel)
	}
	if cfg == nil {
		return ""
	}

	switch normalizeProvider(provider) {
	case "anthropic":
		if cfg.AnthropicModel != "" {
			return cfg.AnthropicModel
		}
	case "openai":
		if cfg.OpenAIModel != "" {
			return cfg.OpenAIModel
		}
	case "gemini":
		if cfg.GeminiModel != "" {
			return cfg.GeminiModel
		}
	case "groq":
		if cfg.GroqModel != "" {
			return cfg.GroqModel
		}
	case "openrouter":
		if cfg.OpenRouterModel != "" {
			return cfg.OpenRouterModel
		}
	}

	if cfg.DefaultModel != "" {
		return cfg.DefaultModel
	}
	return ""
}

// ResolveProvider chooses provider with precedence:
//  1. --provider flag
//  2. config default_provider
//  3. first provider that has an available key in env/config
//  4. anthropic (legacy fallback)
func ResolveProvider(flagProvider string, cfg *Config) string {
	if p := normalizeProvider(flagProvider); p != "" {
		return p
	}
	if cfg != nil {
		if p := normalizeProvider(cfg.DefaultProvider); p != "" {
			return p
		}
	}
	for _, p := range []string{"anthropic", "openai", "gemini", "groq", "openrouter"} {
		if keys, _ := ResolveAPIKeys(p, cfg); len(keys) > 0 {
			return p
		}
	}
	return "anthropic"
}

func ProviderEnvName(p string) string {
	switch normalizeProvider(p) {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "gemini":
		return "GEMINI_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	default:
		return "API_KEY"
	}
}

func ValidProvider(p string) bool {
	switch normalizeProvider(p) {
	case "anthropic", "openai", "gemini", "groq", "openrouter":
		return true
	default:
		return false
	}
}

func normalizeProvider(p string) string {
	return strings.ToLower(strings.TrimSpace(p))
}

// MaskKey shows only the first 7 and last 4 characters of an API key.
func MaskKey(k string) string {
	if len(k) <= 12 {
		return "****"
	}
	return k[:7] + "..." + k[len(k)-4:]
}