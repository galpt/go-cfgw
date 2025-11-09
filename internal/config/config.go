package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration for the tool.
type Config struct {
	APIToken         string
	APIKey           string // optional legacy global API key
	AccountID        string
	AccountEmail     string
	APIHost          string
	AllowURLs        []string
	BlockURLs        []string
	ListItemSize     int
	DryRun           bool
	BlockPageEnabled bool
	BlockBasedOnSNI  bool
	DiscordWebhook   string
}

// LoadFromEnv reads configuration from environment variables and loads a local .env file if present.
func LoadFromEnv() (*Config, error) {
	// Load .env if present (no-op if not found)
	_ = godotenv.Load()

	// Prefer token, fall back to API key
	token := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	key := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_KEY"))
	account := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	acctEmail := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_EMAIL"))

	// Support legacy Node env var name CLOUDFLARE_LIST_ITEM_LIMIT as alias
	listItemSize := 1000
	if s := os.Getenv("CLOUDFLARE_LIST_ITEM_SIZE"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			listItemSize = v
		}
	} else if s := os.Getenv("CLOUDFLARE_LIST_ITEM_LIMIT"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			listItemSize = v
		}
	}

	if token == "" && key == "" {
		return nil, errors.New("one of CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_KEY is required")
	}
	if account == "" {
		return nil, errors.New("CLOUDFLARE_ACCOUNT_ID is required")
	}

	apiHost := os.Getenv("CLOUDFLARE_API_HOST")
	if apiHost == "" {
		apiHost = "https://api.cloudflare.com/client/v4"
	}

	allow := readMultiEnv("ALLOWLIST_URLS")
	// Node allowed USER_DEFINED_ALLOWLIST_URLS but also ALLOWLIST_URLS; support both
	if len(allow) == 0 {
		allow = readMultiEnv("USER_DEFINED_ALLOWLIST_URLS")
	}
	block := readMultiEnv("BLOCKLIST_URLS")
	if len(block) == 0 {
		block = readMultiEnv("USER_DEFINED_BLOCKLIST_URLS")
	}

	dry := false
	if v := os.Getenv("DRY_RUN"); v == "1" || strings.ToLower(v) == "true" {
		dry = true
	}

	bpe := false
	if v := os.Getenv("BLOCK_PAGE_ENABLED"); v == "1" || strings.ToLower(v) == "true" {
		bpe = true
	}

	bsni := false
	if v := os.Getenv("BLOCK_BASED_ON_SNI"); v == "1" || strings.ToLower(v) == "true" {
		bsni = true
	}

	return &Config{
		APIToken:         token,
		APIKey:           key,
		AccountID:        account,
		AccountEmail:     acctEmail,
		APIHost:          apiHost,
		AllowURLs:        allow,
		BlockURLs:        block,
		ListItemSize:     listItemSize,
		DryRun:           dry,
		BlockPageEnabled: bpe,
		BlockBasedOnSNI:  bsni,
		DiscordWebhook:   strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL")),
	}, nil
}

func readMultiEnv(name string) []string {
	v := os.Getenv(name)
	if v == "" {
		return nil
	}
	// Accept newline or comma separated
	out := []string{}
	// Normalize windows newlines and trim
	for _, line := range strings.Split(strings.ReplaceAll(v, "\r", ""), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			// Allow comma-separated values on a single line as well
			if strings.Contains(s, ",") {
				for _, p := range strings.Split(s, ",") {
					if q := strings.TrimSpace(p); q != "" {
						out = append(out, q)
					}
				}
			} else {
				out = append(out, s)
			}
		}
	}
	return out
}
