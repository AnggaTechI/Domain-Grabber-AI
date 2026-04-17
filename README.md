# Domain Grabber AI

<p align="center">
  <img src="https://capsule-render.vercel.app/api?type=waving&height=220&text=Domain%20Grabber%20AI&fontAlign=50&fontAlignY=38&color=0:0f172a,100:2563eb&fontColor=ffffff&desc=AI-powered%20CLI%20tool%20to%20discover,%20filter,%20and%20collect%20real%20domains&descAlign=50&descAlignY=60" />
</p>

<p align="center">
  <img src="https://readme-typing-svg.herokuapp.com?font=Fira+Code&pause=1000&center=true&vCenter=true&width=980&lines=AI-powered+CLI+for+domain+discovery;Generate+real+domains+from+natural+language+queries;Supports+multi-provider+and+multi-API-key+rotation;Filter%2C+normalize%2C+deduplicate%2C+and+store+results" alt="Typing SVG" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-CLI-00ADD8?style=for-the-badge&logo=go&logoColor=white" />
  <img src="https://img.shields.io/badge/AI-Multi_Provider-111827?style=for-the-badge" />
  <img src="https://img.shields.io/badge/Multi_API_Keys-Supported-2563eb?style=for-the-badge" />
  <img src="https://img.shields.io/badge/Status-Active-16a34a?style=for-the-badge" />
</p>

<p align="center">
  <a href="https://github.com/AnggaTechI/Domain-Grabber-AI/releases/download/v1.0.0/domgrab.exe">
    <img src="https://img.shields.io/badge/Download-Windows-2563eb?style=for-the-badge&logo=windows&logoColor=white" />
  </a>
  <a href="https://github.com/AnggaTechI/Domain-Grabber-AI/releases/download/v1.0.0/domgrab">
    <img src="https://img.shields.io/badge/Download-Linux-111827?style=for-the-badge&logo=linux&logoColor=white" />
  </a>
</p>

<p align="center">
  <b>AI-powered CLI tool to discover, filter, and collect real domains from natural language queries.</b>
</p>

---

> [!IMPORTANT]
> **Precompiled binaries are ready.**
>
> - **Windows**: [`domgrab.exe`](https://github.com/AnggaTechI/Domain-Grabber-AI/releases/download/v1.0.0/domgrab.exe)
> - **Linux**: [`domgrab`](https://github.com/AnggaTechI/Domain-Grabber-AI/releases/download/v1.0.0/domgrab)
>
> Download them from the **Releases** page and run directly without building from source.

---

## What's New

- Added **multi API key support**
- Added **automatic key rotation** when a provider hits rate limit or quota
- Added **cooldown handling** for temporarily exhausted keys
- Added **automatic provider selection** from available configured keys
- Added **per-provider model configuration**
- Improved CLI output and provider visibility

---

## Overview

**Domain Grabber AI** helps you turn a simple natural language prompt into a clean list of real domains.

Instead of manually searching and collecting domains one by one, you can ask for things like:

- universities in Indonesia
- government domains from Brazil
- educational institutions in Germany
- startup companies in Southeast Asia
- real estate companies in Singapore

The tool will:

- query the selected AI provider
- extract domain candidates from raw model output
- normalize and validate domains
- apply TLD filters if needed
- remove duplicates automatically
- rotate API keys when needed
- save only fresh results into your master list

It is useful for domain research, niche dataset building, lead collection, and automation workflows.

---

## Highlights

- Natural language domain discovery
- Multi-provider AI support
- **Multi API key support per provider**
- **Automatic key rotation and cooldown handling**
- Per-provider model configuration
- Automatic provider selection from available API keys
- Domain normalization and validation
- TLD filtering support
- Duplicate prevention
- Persistent master list storage
- Built-in `list`, `stats`, and `config` commands
- Portable `domgrab.json` config file
- Ready-to-use Windows and Linux binaries

---

## Supported Providers

- Anthropic
- OpenAI
- Gemini
- Groq
- OpenRouter

Each provider can use:

- its own **single API key**
- or **multiple API keys** for rotation
- its own **custom model** through `domgrab.json`

---

## How It Works

```mermaid
flowchart TD
    A[User enters a natural language query] --> B[CLI selects provider and model]
    B --> C[Load available API keys]
    C --> D[Pick active key from keyring]
    D --> E[AI model generates raw domain suggestions]
    E --> F[Extract domain candidates]
    F --> G[Normalize and validate domains]
    G --> H[Apply TLD filter if enabled]
    H --> I[Remove duplicates]
    I --> J[Append fresh domains to master.txt]
    E --> K{Rate limit or quota hit?}
    K -- Yes --> L[Cooldown current key and rotate to next key]
    L --> D
    K -- No --> F
```

---

## Quick Start

### Windows

```bash
domgrab.exe config set gemini_api_key YOUR_GEMINI_KEY
domgrab.exe config set default_provider gemini
domgrab.exe config set gemini_model gemini-3-flash-preview
domgrab.exe grab --query "universities in Indonesia" --target 500 --batch 100 --tld ac.id
```

### Linux

```bash
chmod +x domgrab
./domgrab config set gemini_api_key YOUR_GEMINI_KEY
./domgrab config set default_provider gemini
./domgrab config set gemini_model gemini-3-flash-preview
./domgrab grab --query "universities in Indonesia" --target 500 --batch 100 --tld ac.id
```

---

## Download Precompiled Binaries

### Direct Downloads

- [Windows Binary](https://github.com/AnggaTechI/Domain-Grabber-AI/releases/download/v1.0.0/domgrab.exe)
- [Linux Binary](https://github.com/AnggaTechI/Domain-Grabber-AI/releases/download/v1.0.0/domgrab)

### Usage

**Windows**
```bash
domgrab.exe grab --query "universities in Indonesia" --target 500 --batch 100 --tld ac.id
```

**Linux**
```bash
chmod +x domgrab
./domgrab grab --query "universities in Indonesia" --target 500 --batch 100 --tld ac.id
```

---

## Project Structure

```bash
Domain-Grabber-AI/
├── main.go
├── go.mod
├── README.md
│
├── internal/
│   ├── core/
│   │   ├── config.go
│   │   ├── provider.go
│   │   ├── keyring.go
│   │   ├── domain.go
│   │   └── store.go
│   │
│   └── cli/
│       ├── grab.go
│       ├── config_cmd.go
│       └── commands.go
```

---

## Installation

### Clone the repository

```bash
git clone https://github.com/AnggaTechI/Domain-Grabber-AI.git
cd Domain-Grabber-AI
```

### Build from source

**Windows**
```bash
go build -o domgrab.exe .
```

**Linux / macOS**
```bash
go build -o domgrab .
```

---

## Basic Usage

```bash
domgrab <command> [flags]
```

### Commands

- `grab` — Grab domains via AI from a natural language query
- `list` — Show domains in the master list
- `stats` — Show domain statistics
- `config` — Manage API keys, models, and defaults
- `version` — Show version information
- `help` — Show help message

---

## Multi API Keys

Domain Grabber AI now supports **multiple API keys per provider**.

This is useful when:

- one key reaches rate limit
- one key is temporarily exhausted
- you want more stable long-running batches
- you want automatic failover without manual key switching

### Example concept

If you configure multiple Gemini keys, the tool can:

1. use key #1
2. detect quota/rate limit
3. put key #1 on cooldown
4. rotate to key #2
5. continue grabbing without stopping immediately

---

## Configuration

Configuration is stored in:

```bash
./domgrab.json
```

### Example Config (single key + multi key ready)

```json
{
  "anthropic_api_key": "",
  "openai_api_key": "",
  "gemini_api_key": "YOUR_PRIMARY_GEMINI_KEY",
  "groq_api_key": "",
  "openrouter_api_key": "",

  "anthropic_api_keys": [],
  "openai_api_keys": [],
  "gemini_api_keys": [
    "YOUR_GEMINI_KEY_1",
    "YOUR_GEMINI_KEY_2",
    "YOUR_GEMINI_KEY_3"
  ],
  "groq_api_keys": [],
  "openrouter_api_keys": [],

  "default_provider": "gemini",
  "default_model": "",
  "default_output": "master.txt",

  "anthropic_model": "claude-opus-4-7",
  "openai_model": "gpt-4o",
  "gemini_model": "gemini-3-flash-preview",
  "groq_model": "llama-3.3-70b-versatile",
  "openrouter_model": "meta-llama/llama-3.3-70b-instruct:free"
}
```

---

## Provider Resolution Logic

**Provider selection order**
1. `--provider` flag
2. `default_provider` in config
3. first available provider with a valid API key
4. fallback to `anthropic`

**API key resolution order**
1. `--api-key` flag
2. environment variable
3. single key in `domgrab.json`
4. multi-key list in `domgrab.json`

**Model resolution order**
1. `--model` flag
2. provider-specific model from config
3. `default_model`
4. provider fallback model

---

## Example Workflow

### Input

```bash
domgrab.exe grab --query "universities in Indonesia" --target 500 --batch 100 --tld ac.id
```

### Process

1. The CLI loads your configuration.
2. It selects the provider and model.
3. It loads available API keys.
4. It picks the active key from the keyring.
5. It sends the prompt to the AI provider.
6. The model returns raw text.
7. The tool extracts valid domains.
8. The TLD filter is applied.
9. Duplicate domains are removed.
10. New results are saved to `master.txt`.
11. If rate-limited, the tool rotates to another key automatically.

### Sample Output

```txt
ugm.ac.id
ui.ac.id
itb.ac.id
unair.ac.id
ipb.ac.id
```

---

## More Examples

### Grab Indonesian university domains

```bash
domgrab.exe grab --query "universities in Indonesia" --target 500 --batch 100 --tld ac.id
```

### Grab Indonesian academic and government domains

```bash
domgrab.exe grab --query "universities and government institutions in Indonesia" --target 500 --batch 100 --tld "ac.id,go.id"
```

### Grab Brazilian government domains

```bash
domgrab.exe grab --query "government domains from Brazil" --target 500 --batch 100 --tld gov.br
```

### Grab German university domains

```bash
domgrab.exe grab --provider gemini --query "universities in Germany" --target 500 --batch 100
```

### Use a custom output file

```bash
domgrab.exe grab --query "tech companies in Singapore" --target 500 --batch 100 --output singapore.txt
```

### List domains containing a keyword

```bash
domgrab.exe list --filter uin
```

### Show TLD stats

```bash
domgrab.exe stats
```

---

## Example CLI Output

```txt
═══════════════════════════════════════════
 domgrab v1.0.0
 author   : AnggaTechI
 github   : https://github.com/AnggaTechI
═══════════════════════════════════════════
 provider : gemini (rotating 3 keys, from config)
 model    : gemini-3-flash-preview
 query    : universities in Indonesia
 target   : 500 new domains
 batch    : 100 per request
 output   : master.txt (currently 0 domains)
 tld      : ac.id
═══════════════════════════════════════════
```

---

## Why Use Domain Grabber AI?

- Build domain datasets faster
- Discover niche websites by topic or country
- Collect institutional domains in bulk
- Reduce manual provider/key switching
- Keep everything in one clean master list
- Automate repetitive domain research tasks
- Combine AI generation with your own filtering strategy

---

## Notes

- `master.txt` is the default output file
- Domains are normalized before saving
- Duplicate domains are skipped automatically
- TLD filters are optional
- Different AI providers may have different rate limits
- Multi-key rotation helps reduce interruptions during large batches
- Results depend on the prompt quality and selected model

---

## Repository

GitHub Repository: https://github.com/AnggaTechI/Domain-Grabber-AI

---

## Author

**AnggaTechI**  
GitHub: https://github.com/AnggaTechI

---

## License

This project is released under the MIT License.

---

<p align="center">
  <img src="https://capsule-render.vercel.app/api?type=waving&section=footer&height=120&color=0:2563eb,100:0f172a" />
</p>
