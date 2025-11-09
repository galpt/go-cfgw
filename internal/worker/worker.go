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

	// Create lists in chunks for block and allow
	if len(block) > 0 {
		if err := w.createListsInChunks(ctx, client, cfg, "CGPS Block List", block); err != nil {
			return err
		}
	}
	if len(allow) > 0 {
		if err := w.createListsInChunks(ctx, client, cfg, "CGPS Allow List", allow); err != nil {
			return err
		}
	}

	// Build a simple wirefilter expression that matches DNS traffic for lists.
	// Cloudflare wirefilter specifics may vary; keep conservative and let user inspect.
	// We'll create/update a rule named "CGPS Filter Lists" with dns filter.
	trafficExpr := map[string]any{"lists": "auto"}
	filters := []string{"dns"}
	if err := client.CreateOrUpdateRule(ctx, "CGPS Filter Lists", trafficExpr, filters, true); err != nil {
		return fmt.Errorf("create rule: %w", err)
	}

	// brief cooldown to let API settle
	time.Sleep(1 * time.Second)
	return nil
}

func (w *Worker) createListsInChunks(ctx context.Context, client *cf.Client, cfg *config.Config, baseName string, items []string) error {
	size := cfg.ListItemSize
	total := len(items)
	chunks := int(math.Ceil(float64(total) / float64(size)))
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
		if _, err := client.CreateList(ctx, name, payload); err != nil {
			return fmt.Errorf("create list %s: %w", name, err)
		}
		w.opts.Logger.Infof("created %s (%d items)", name, len(payload))
	}
	return nil
}
