package cf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/galpt/go-cfgw/internal/config"
	"github.com/galpt/go-cfgw/internal/logging"
)

// Client is a small Cloudflare Gateway API client with retry/backoff and rate-limit handling.
// The HTTP client is thread-safe. Currently used single-threaded, but safe for concurrent use.
type Client struct {
	http    *http.Client
	token   string
	account string
	host    string
	logger  *logging.Logger
}

func NewClient(cfg *config.Config, logger *logging.Logger) *Client {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &Client{http: httpClient, token: cfg.APIToken, account: cfg.AccountID, host: cfg.APIHost, logger: logger}
}

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyBytes = b
	}

	var out []byte
	operation := func() error {
		reqBody := bytes.NewReader(bodyBytes)
		url := strings.TrimRight(c.host, "/") + "/accounts/" + c.account + "/gateway" + path
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return backoff.Permanent(err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			c.logger.Debugf("http.do error: %v", err)
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 429 {
			// Respect Retry-After if present
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					wait := time.Duration(secs)*time.Second + 500*time.Millisecond
					c.logger.Infof("rate limited, waiting %v before retrying", wait)
					time.Sleep(wait)
				}
			} else {
				// default cooldown
				c.logger.Infof("rate limited (429), backing off")
			}
			return fmt.Errorf("rate limited: 429")
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
		}

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		out = b
		return nil
	}

	// Exponential backoff with max elapsed time
	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = 2 * time.Minute
	bo := backoff.WithContext(eb, ctx)

	err := backoff.Retry(operation, bo)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetLists returns the zero trust lists
func (c *Client) GetLists(ctx context.Context) (map[string]any, error) {
	b, err := c.doRequestWithRetry(ctx, "GET", "/lists", nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateList creates a Zero Trust list with provided items (items are objects with "value" property)
func (c *Client) CreateList(ctx context.Context, name string, items []map[string]any) (map[string]any, error) {
	body := map[string]any{"name": name, "type": "DOMAIN", "items": items}
	b, err := c.doRequestWithRetry(ctx, "POST", "/lists", body)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteList deletes a list by ID
func (c *Client) DeleteList(ctx context.Context, id any) error {
	s := fmt.Sprintf("%v", id)
	_, err := c.doRequestWithRetry(ctx, "DELETE", "/lists/"+s, nil)
	return err
}

// GetRules returns the gateway rules
func (c *Client) GetRules(ctx context.Context) (map[string]any, error) {
	b, err := c.doRequestWithRetry(ctx, "GET", "/rules", nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteRule deletes a rule by ID
func (c *Client) DeleteRule(ctx context.Context, id any) error {
	s := fmt.Sprintf("%v", id)
	_, err := c.doRequestWithRetry(ctx, "DELETE", "/rules/"+s, nil)
	return err
}

// DeleteAllOldRules deletes all rules matching the old naming patterns (CGPS and Go-CFGW).
// This ensures a clean slate before creating new rules.
func (c *Client) DeleteAllOldRules(ctx context.Context) error {
	rulesResp, err := c.GetRules(ctx)
	if err != nil {
		return fmt.Errorf("get rules: %w", err)
	}

	deleted := 0
	if res, ok := rulesResp["result"].([]any); ok {
		for _, r := range res {
			if rmap, ok := r.(map[string]any); ok {
				ruleName, _ := rmap["name"].(string)
				// Delete both old CGPS rules (DNS and SNI) and any existing Go-CFGW rules
				if strings.Contains(ruleName, "CGPS Filter Lists") ||
					strings.Contains(ruleName, "Go-CFGW Filter Lists") {
					id := rmap["id"]
					c.logger.Infof("Deleting old rule: %s", ruleName)
					if err := c.DeleteRule(ctx, id); err != nil {
						c.logger.Warnf("Failed to delete rule %s: %v", ruleName, err)
						// Continue deleting others
					} else {
						deleted++
					}
				}
			}
		}
	}

	if deleted > 0 {
		c.logger.Infof("Deleted %d old rule(s)", deleted)
	} else {
		c.logger.Infof("No old rules found to delete")
	}
	return nil
}

// DeleteAllOldLists deletes all lists matching the old naming patterns (CGPS and Go-CFGW).
// This ensures a clean slate before creating new lists.
func (c *Client) DeleteAllOldLists(ctx context.Context) error {
	listsResp, err := c.GetLists(ctx)
	if err != nil {
		return fmt.Errorf("get lists: %w", err)
	}

	deleted := 0
	if res, ok := listsResp["result"].([]any); ok {
		for _, l := range res {
			if lmap, ok := l.(map[string]any); ok {
				listName, _ := lmap["name"].(string)
				// Delete both old CGPS lists and any existing Go-CFGW lists
				// Use Contains to catch all variations: "CGPS List", "CGPS Block List", etc.
				if strings.Contains(listName, "CGPS") ||
					strings.HasPrefix(listName, "Go-CFGW Block List") ||
					strings.HasPrefix(listName, "Go-CFGW Allow List") {
					id := lmap["id"]
					c.logger.Infof("Deleting old list: %s", listName)
					if err := c.DeleteList(ctx, id); err != nil {
						c.logger.Warnf("Failed to delete list %s: %v", listName, err)
						// Continue deleting others
					} else {
						deleted++
					}
				}
			}
		}
	}

	if deleted > 0 {
		c.logger.Infof("Deleted %d old list(s)", deleted)
	} else {
		c.logger.Infof("No old lists found to delete")
	}
	return nil
}

// CreateOrUpdateRule creates or updates a rule. If rule with name exists, updates it.
func (c *Client) CreateOrUpdateRule(ctx context.Context, name string, traffic any, filters []string, blockPageEnabled bool) error {
	// Query existing rules
	rulesResp, err := c.GetRules(ctx)
	if err != nil {
		return err
	}
	if res, ok := rulesResp["result"].([]any); ok {
		for _, r := range res {
			if rmap, ok := r.(map[string]any); ok {
				if rmap["name"] == name {
					id := rmap["id"]
					// Update
					body := map[string]any{"name": name, "description": "Filter lists created by go-cfgw. Avoid editing this rule. Changing the name of this rule will break the script.", "enabled": true, "action": "block", "rule_settings": map[string]any{"block_page_enabled": blockPageEnabled, "block_reason": "Blocked by go-cfgw, check your filter lists if this was a mistake."}, "filters": filters, "traffic": traffic}
					_, err := c.doRequestWithRetry(ctx, "PUT", "/rules/"+fmt.Sprintf("%v", id), body)
					return err
				}
			}
		}
	}
	// Create
	body := map[string]any{"name": name, "description": "Filter lists created by go-cfgw. Avoid editing this rule. Changing the name of this rule will break the script.", "enabled": true, "action": "block", "rule_settings": map[string]any{"block_page_enabled": blockPageEnabled, "block_reason": "Blocked by go-cfgw, check your filter lists if this was a mistake."}, "filters": filters, "traffic": traffic}
	_, err = c.doRequestWithRetry(ctx, "POST", "/rules", body)
	return err
}
