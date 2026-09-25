// Command loadtest runs repeatable HTTP load scenarios against a deployed
// CampusHub instance. It deliberately has no third-party dependency so CI or
// an operator can run it with `go run`.
//
// It never creates users or activities. Write scenarios are explicit and need
// a pre-provisioned bearer token and request body, preventing accidental load
// against production business data.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	Latency time.Duration
	Status  int
	Err     error
}

func main() {
	var baseURL, scenario, path, method, body, token string
	var expectedStatus string
	var concurrency int
	var duration time.Duration
	flag.StringVar(&baseURL, "base-url", env("CAMPUSHUB_LOAD_BASE_URL", "http://127.0.0.1:8080"), "CampusHub base URL")
	flag.StringVar(&scenario, "scenario", env("CAMPUSHUB_LOAD_SCENARIO", "activity-list"), "activity-list, activity-search, or custom")
	flag.StringVar(&path, "path", env("CAMPUSHUB_LOAD_PATH", ""), "custom API path")
	flag.StringVar(&method, "method", env("CAMPUSHUB_LOAD_METHOD", "GET"), "custom HTTP method")
	flag.StringVar(&body, "body", env("CAMPUSHUB_LOAD_BODY", ""), "custom JSON request body")
	flag.StringVar(&token, "token", env("CAMPUSHUB_LOAD_TOKEN", ""), "optional bearer token")
	flag.StringVar(&expectedStatus, "expected-status", env("CAMPUSHUB_LOAD_EXPECTED_STATUS", "2xx"), "expected statuses: 2xx or comma-separated codes")
	flag.IntVar(&concurrency, "concurrency", envInt("CAMPUSHUB_LOAD_CONCURRENCY", 20), "parallel workers")
	flag.DurationVar(&duration, "duration", envDuration("CAMPUSHUB_LOAD_DURATION", 30*time.Second), "test duration")
	flag.Parse()
	if concurrency < 1 || duration <= 0 {
		fail("concurrency and duration must be positive")
	}

	requestPath, requestMethod, requestBody := resolveScenario(scenario, path, method, body)
	if requestPath == "" {
		fail("custom scenario requires -path")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	results := make(chan result, concurrency*4)
	collected := make([]result, 0, concurrency*1024)
	var collector sync.WaitGroup
	collector.Add(1)
	go func() {
		defer collector.Done()
		for item := range results {
			collected = append(collected, item)
		}
	}()
	var requests uint64
	var workers sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for ctx.Err() == nil {
				started := time.Now()
				status, err := request(ctx, client, strings.TrimRight(baseURL, "/")+requestPath, requestMethod, requestBody, token)
				// A request cancelled solely because the scheduled test window
				// closed is not an application transport error.
				if ctx.Err() != nil {
					return
				}
				results <- result{Latency: time.Since(started), Status: status, Err: err}
				atomic.AddUint64(&requests, 1)
			}
		}()
	}
	<-ctx.Done()
	workers.Wait()
	close(results)
	collector.Wait()
	if !report(scenario, concurrency, duration, atomic.LoadUint64(&requests), expectedStatus, collected) {
		os.Exit(1)
	}
}

func resolveScenario(scenario, path, method, body string) (string, string, string) {
	switch scenario {
	case "activity-list":
		return "/api/v1/activities?page=1&page_size=10&sort=start_time", http.MethodGet, ""
	case "activity-search":
		return "/api/v1/activities/search?keyword=campus&page=1&page_size=10&sort=hot", http.MethodGet, ""
	case "custom":
		return path, strings.ToUpper(method), body
	default:
		fail("unknown scenario: " + scenario)
	}
	return "", "", ""
}

func request(ctx context.Context, client *http.Client, url, method, body, token string) (int, error) {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func report(scenario string, concurrency int, duration time.Duration, total uint64, expected string, results []result) bool {
	latencies := make([]time.Duration, 0, total)
	statuses := map[int]int{}
	errors := 0
	for _, item := range results {
		latencies = append(latencies, item.Latency)
		if item.Err != nil {
			errors++
		} else {
			statuses[item.Status]++
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	summary := map[string]any{
		"scenario": scenario, "concurrency": concurrency, "duration": duration.String(), "requests": total,
		"rps": float64(total) / duration.Seconds(), "transport_errors": errors, "status_codes": statuses,
		"p50_ms": percentile(latencies, .50).Milliseconds(), "p95_ms": percentile(latencies, .95).Milliseconds(), "p99_ms": percentile(latencies, .99).Milliseconds(),
	}
	summary["expected_status"] = expected
	failed := errors > 0
	for status := range statuses {
		if !matchesExpected(status, expected) {
			failed = true
		}
	}
	summary["passed"] = !failed
	encoded, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(encoded))
	return !failed
}

func matchesExpected(status int, expected string) bool {
	if expected == "2xx" {
		return status >= 200 && status < 300
	}
	for _, item := range strings.Split(expected, ",") {
		if strings.TrimSpace(item) == fmt.Sprint(status) {
			return true
		}
	}
	return false
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * p)
	return values[index]
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envInt(key string, fallback int) int {
	var value int
	if _, err := fmt.Sscanf(os.Getenv(key), "%d", &value); err == nil && value > 0 {
		return value
	}
	return fallback
}
func envDuration(key string, fallback time.Duration) time.Duration {
	if value, err := time.ParseDuration(os.Getenv(key)); err == nil && value > 0 {
		return value
	}
	return fallback
}
func fail(message string) { fmt.Fprintln(os.Stderr, "loadtest:", message); os.Exit(2) }
