// Package analytics ships redirect-service click events to OpenObserve
// so the shared job-funnel dashboards have the full chain (page view →
// apply click → redirect) in a single observability backend.
//
// This is a deliberate duplicate of stawi.jobs/pkg/analytics, kept
// local so the two services can be deployed independently. When a
// third service needs the same client it should be promoted to a
// shared module under antinvestor/common.
package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client batches events in memory and flushes to OpenObserve's
// HTTP ingest API. Safe to use from multiple goroutines. A nil Client
// is a no-op on every method so callers don't need to branch.
type Client struct {
	baseURL  string
	org      string
	username string
	password string
	http     *http.Client

	maxBatch int
	flushInt time.Duration

	mu      sync.Mutex
	buckets map[string][]map[string]any
	stopCh  chan struct{}
	stopped bool
	wg      sync.WaitGroup
}

type Config struct {
	BaseURL       string
	Org           string
	Username      string
	Password      string
	MaxBatchSize  int
	FlushInterval time.Duration
	HTTPTimeout   time.Duration
}

// New returns a Client and starts its background flusher. Nil when
// cfg.BaseURL is empty.
func New(cfg Config) *Client {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil
	}
	if cfg.Org == "" {
		cfg.Org = "default"
	}
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 10 * time.Second
	}

	c := &Client{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		org:      cfg.Org,
		username: cfg.Username,
		password: cfg.Password,
		http:     &http.Client{Timeout: cfg.HTTPTimeout},
		maxBatch: cfg.MaxBatchSize,
		flushInt: cfg.FlushInterval,
		buckets:  make(map[string][]map[string]any),
		stopCh:   make(chan struct{}),
	}
	c.wg.Add(1)
	go c.runFlusher()
	return c
}

// Send enqueues one event. Triggers an immediate flush when the batch
// for that stream reaches MaxBatchSize.
func (c *Client) Send(ctx context.Context, stream string, event map[string]any) {
	if c == nil || c.stopped || stream == "" || len(event) == 0 {
		return
	}
	if _, ok := event["_timestamp"]; !ok {
		event["_timestamp"] = time.Now().UnixMicro()
	}

	c.mu.Lock()
	c.buckets[stream] = append(c.buckets[stream], event)
	shouldFlush := len(c.buckets[stream]) >= c.maxBatch
	batch := c.buckets[stream]
	if shouldFlush {
		c.buckets[stream] = nil
	}
	c.mu.Unlock()

	if shouldFlush {
		go func(stream string, batch []map[string]any) {
			_ = c.flush(ctx, stream, batch)
		}(stream, batch)
	}
}

// Close drains remaining events and stops the flusher goroutine.
func (c *Client) Close(ctx context.Context) error {
	if c == nil || c.stopped {
		return nil
	}
	c.stopped = true
	close(c.stopCh)
	c.wg.Wait()

	c.mu.Lock()
	pending := c.buckets
	c.buckets = nil
	c.mu.Unlock()

	var firstErr error
	for stream, batch := range pending {
		if err := c.flush(ctx, stream, batch); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Client) runFlusher() {
	defer c.wg.Done()
	t := time.NewTicker(c.flushInt)
	defer t.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
			c.flushAll()
		}
	}
}

func (c *Client) flushAll() {
	c.mu.Lock()
	snapshot := c.buckets
	c.buckets = make(map[string][]map[string]any, len(snapshot))
	c.mu.Unlock()

	for stream, batch := range snapshot {
		if len(batch) == 0 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = c.flush(ctx, stream, batch)
		cancel()
	}
}

func (c *Client) flush(ctx context.Context, stream string, batch []map[string]any) error {
	if len(batch) == 0 {
		return nil
	}
	body, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("analytics: marshal batch: %w", err)
	}

	url := fmt.Sprintf("%s/api/%s/%s/_json", c.baseURL, c.org, stream)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("analytics: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("analytics: do %s: %w", stream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("analytics: %s status %d", stream, resp.StatusCode)
	}
	return nil
}
