package downloader

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/galpt/go-cfgw/internal/config"
	"github.com/galpt/go-cfgw/internal/logging"
)

// Options for downloader.
type Options struct {
	Client *http.Client
	Logger *logging.Logger
}

type Downloader struct {
	client *http.Client
	logger *logging.Logger
}

func New(o *Options) *Downloader {
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Downloader{client: client, logger: o.Logger}
}

// DownloadAndProcess downloads allow and block lists, normalizes and dedupes entries.
func (d *Downloader) DownloadAndProcess(ctx context.Context, cfg *config.Config) (allow []string, block []string, err error) {
	allowSet := map[string]struct{}{}
	blockSet := map[string]struct{}{}

	// If no URLs were provided, return empty lists (caller may decide defaults)
	for _, url := range cfg.AllowURLs {
		if err := d.fetchIntoSet(ctx, url, allowSet); err != nil {
			return nil, nil, err
		}
	}
	for _, url := range cfg.BlockURLs {
		if err := d.fetchIntoSet(ctx, url, blockSet); err != nil {
			return nil, nil, err
		}
	}

	for k := range allowSet {
		allow = append(allow, k)
	}
	for k := range blockSet {
		block = append(block, k)
	}

	return allow, block, nil
}

var commentPrefix = regexp.MustCompile(`^\s*(#|//|!|/\*)`)
var hostPattern = regexp.MustCompile(`^((?=[a-z0-9-]{1,63}\.)[a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,63}$`)

func (d *Downloader) fetchIntoSet(ctx context.Context, url string, dest map[string]struct{}) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := d.client.Do(req)
	if err != nil {
		d.logger.Errorf("download %s: %v", url, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		d.logger.Errorf("non-2xx response %d from %s", resp.StatusCode, url)
		return err
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" || commentPrefix.MatchString(line) {
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			continue
		}
		// Basic normalization similar to original script
		normalized := normalizeLine(line)
		if hostPattern.MatchString(normalized) {
			dest[normalized] = struct{}{}
		}
		if err == io.EOF {
			break
		}
	}
	return nil
}

func normalizeLine(line string) string {
	s := line
	// remove common hosts prefixes like 0.0.0.0 or 127.0.0.1
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0.0.0.0 ")
	s = strings.TrimPrefix(s, "127.0.0.1 ")
	s = strings.TrimPrefix(s, "::1 ")
	s = strings.TrimPrefix(s, "||")
	s = strings.TrimPrefix(s, "*.")
	s = strings.TrimPrefix(s, "^")
	// Remove any trailing metadata used by some lists
	s = strings.Split(s, " ")[0]
	s = strings.Trim(s, "\t\r\n ")
	return strings.ToLower(s)
}
