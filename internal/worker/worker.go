package worker

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/galpt/go-cfgw/internal/cf"
	"github.com/galpt/go-cfgw/internal/config"
	"github.com/galpt/go-cfgw/internal/logging"
)

type Options struct {
	Logger *logging.Logger
	DryRun bool
}

type Worker struct {
	opts Options
}

func New(opts Options) *Worker { return &Worker{opts: opts} }

// Run orchestrates updating Cloudflare lists and rules.
func (w *Worker) Run(ctx context.Context, cfg *config.Config, allow []string, block []string) error {
	client := cf.NewClient(cfg, w.opts.Logger)

	// Check total item limit
	totalItems := len(allow) + len(block)
	if totalItems > cfg.ListItemLimit {
		w.opts.Logger.Infof("WARNING: Total items (%d) exceeds CLOUDFLARE_LIST_ITEM_LIMIT (%d)", totalItems, cfg.ListItemLimit)
		w.opts.Logger.Infof("Proceeding anyway, but you may hit Cloudflare account limits")
	}

	// Step 1: Clean up all old rules first (both CGPS and Go-CFGW)
	w.opts.Logger.Infof("Cleaning up old rules...")
	if err := client.DeleteAllOldRules(ctx); err != nil {
		return fmt.Errorf("cleanup old rules: %w", err)
	}

	// Step 2: Clean up all old lists (both CGPS and Go-CFGW)
	w.opts.Logger.Infof("Cleaning up old lists...")
	if err := client.DeleteAllOldLists(ctx); err != nil {
		return fmt.Errorf("cleanup old lists: %w", err)
	}

	// Brief pause to let API settle after deletions
	time.Sleep(2 * time.Second)

	// Step 3: Create new blocklist chunks
	var createdListIDs []string
	if len(block) > 0 {
		w.opts.Logger.Infof("Creating blocklists with %d total entries...", len(block))
		ids, err := w.createListsInChunks(ctx, client, cfg, "Go-CFGW Block List", block)
		if err != nil {
			return fmt.Errorf("create block lists: %w", err)
		}
		createdListIDs = append(createdListIDs, ids...)
	}

	// Step 4: Create allowlist chunks if any
	if len(allow) > 0 {
		w.opts.Logger.Infof("Creating allowlists with %d total entries...", len(allow))
		ids, err := w.createListsInChunks(ctx, client, cfg, "Go-CFGW Allow List", allow)
		if err != nil {
			return fmt.Errorf("create allow lists: %w", err)
		}
		createdListIDs = append(createdListIDs, ids...)
	}

	// Step 5: Build wirefilter expression
	if len(createdListIDs) == 0 {
		w.opts.Logger.Infof("No lists created, skipping rule creation")
		return nil
	}

	w.opts.Logger.Infof("Creating Gateway rule for %d list(s)...", len(createdListIDs))

	// Build wirefilter expression matching Node.js implementation
	// Format: any(dns.domains[*] in $listID1) or any(dns.domains[*] in $listID2) or ...
	wirefilterExpr := ""
	for _, id := range createdListIDs {
		wirefilterExpr += fmt.Sprintf("any(dns.domains[*] in $%s) or ", id)
	}
	// Remove trailing " or "
	wirefilterExpr = wirefilterExpr[:len(wirefilterExpr)-4]

	filters := []string{"dns"}
	if err := client.CreateOrUpdateRule(ctx, "Go-CFGW Filter Lists", wirefilterExpr, filters, cfg.BlockPageEnabled); err != nil {
		return fmt.Errorf("create dns rule: %w", err)
	}

	// Optionally create SNI-based rule if configured
	if cfg.BlockBasedOnSNI {
		w.opts.Logger.Infof("Creating SNI-based rule for %d list(s)...", len(createdListIDs))

		// Build SNI wirefilter expression
		// Format: any(net.sni.domains[*] in $listID1) or any(net.sni.domains[*] in $listID2) or ...
		wirefilterSNIExpr := ""
		for _, id := range createdListIDs {
			wirefilterSNIExpr += fmt.Sprintf("any(net.sni.domains[*] in $%s) or ", id)
		}
		// Remove trailing " or "
		wirefilterSNIExpr = wirefilterSNIExpr[:len(wirefilterSNIExpr)-4]

		sniFilters := []string{"l4"}
		if err := client.CreateOrUpdateRule(ctx, "Go-CFGW Filter Lists - SNI Based Filtering", wirefilterSNIExpr, sniFilters, cfg.BlockPageEnabled); err != nil {
			return fmt.Errorf("create sni rule: %w", err)
		}
	}

	w.opts.Logger.Infof("Successfully updated Cloudflare Gateway!")
	return nil
}

func (w *Worker) createListsInChunks(ctx context.Context, client *cf.Client, cfg *config.Config, baseName string, items []string) ([]string, error) {
	size := cfg.ListItemSize
	total := len(items)
	chunks := int(math.Ceil(float64(total) / float64(size)))

	w.opts.Logger.Infof("Will create %d list(s) with chunk size %d", chunks, size)

	var createdIDs []string
	for i := 0; i < chunks; i++ {
		start := i * size
		end := (i + 1) * size
		if end > total {
			end = total
		}
		chunk := items[start:end]
		// convert to items expected by Cloudflare (value property)
		payload := []map[string]any{}
		for _, v := range chunk {
			payload = append(payload, map[string]any{"value": v})
		}
		name := fmt.Sprintf("%s - Chunk %d", baseName, i+1)

		if w.opts.DryRun {
			w.opts.Logger.Infof("dry-run: would create list %s with %d items", name, len(payload))
			continue
		}

		w.opts.Logger.Infof("Creating list %s with %d items... (%d/%d lists remaining)", name, len(payload), chunks-i, chunks)
		resp, err := client.CreateList(ctx, name, payload)
		if err != nil {
			return nil, fmt.Errorf("create list %s: %w", name, err)
		}

		// Extract the list ID from response
		if result, ok := resp["result"].(map[string]any); ok {
			if id, ok := result["id"].(string); ok {
				createdIDs = append(createdIDs, id)
			}
		}

		w.opts.Logger.Infof("Created %s successfully - %d list(s) remaining", name, chunks-i-1)
	}
	return createdIDs, nil
}
