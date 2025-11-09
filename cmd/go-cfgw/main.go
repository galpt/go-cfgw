package main

import (
	"context"
	"flag"

	"github.com/galpt/go-cfgw/internal/config"
	"github.com/galpt/go-cfgw/internal/downloader"
	"github.com/galpt/go-cfgw/internal/logging"
	"github.com/galpt/go-cfgw/internal/worker"
)

func main() {
	ctx := context.Background()
	// Simple flags for dry-run and debug
	dryRun := flag.Bool("dry-run", false, "Run without sending changes to Cloudflare")
	flag.Parse()

	logger := logging.NewLogger(*dryRun)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Fatalf("config: %v", err)
	}
	if *dryRun {
		logger.Infof("Running in dry-run mode")
	}

	dl := downloader.New(&downloader.Options{Client: nil, Logger: logger})
	// Download and normalize lists (sequential to reduce rate hits)
	logger.Infof("Starting download of lists...")
	allow, block, err := dl.DownloadAndProcess(ctx, cfg)
	if err != nil {
		logger.Fatalf("download: %v", err)
	}
	logger.Infof("Downloaded %d allow entries and %d block entries", len(allow), len(block))

	// Orchestrate Cloudflare updates
	w := worker.New(worker.Options{Logger: logger, DryRun: *dryRun})
	if err := w.Run(ctx, cfg, allow, block); err != nil {
		logger.Fatalf("worker: %v", err)
	}

	logger.Infof("Done")
}
