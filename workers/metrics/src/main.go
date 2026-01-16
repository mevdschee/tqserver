package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mevdschee/tqserver/pkg/worker"
	"github.com/mevdschee/tqtemplate"
)

var tmpl *tqtemplate.Template

// Metric represents a parsed Prometheus metric
type Metric struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
	Help   string            `json:"help,omitempty"`
	Type   string            `json:"type,omitempty"`
}

// MetricsResponse is the JSON response for the API
type MetricsResponse struct {
	Timestamp   string              `json:"timestamp"`
	Metrics     map[string][]Metric `json:"metrics"`
	RawCount    int                 `json:"raw_count"`
	ParsedCount int                 `json:"parsed_count"`
}

// parsePrometheusMetrics parses Prometheus text format into structured metrics
func parsePrometheusMetrics(data string) (map[string][]Metric, int) {
	metrics := make(map[string][]Metric)
	scanner := bufio.NewScanner(strings.NewReader(data))

	// Regex to parse metric lines: metric_name{label="value",...} value
	metricRegex := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)(\{([^}]*)\})?\s+([0-9eE.+-]+|NaN|Inf|-Inf)`)
	labelRegex := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"`)

	helpMap := make(map[string]string)
	typeMap := make(map[string]string)
	count := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse HELP comments
		if strings.HasPrefix(line, "# HELP ") {
			parts := strings.SplitN(line[7:], " ", 2)
			if len(parts) == 2 {
				helpMap[parts[0]] = parts[1]
			}
			continue
		}

		// Parse TYPE comments
		if strings.HasPrefix(line, "# TYPE ") {
			parts := strings.SplitN(line[7:], " ", 2)
			if len(parts) == 2 {
				typeMap[parts[0]] = parts[1]
			}
			continue
		}

		// Skip other comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Parse metric line
		matches := metricRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		name := matches[1]
		labelsStr := matches[3]
		valueStr := matches[4]

		// Parse value
		var value float64
		if valueStr == "NaN" {
			value = 0 // Treat NaN as 0 for charting
		} else if valueStr == "Inf" || valueStr == "+Inf" {
			value = 0 // Treat Inf as 0 for charting
		} else if valueStr == "-Inf" {
			value = 0 // Treat -Inf as 0 for charting
		} else {
			var err error
			value, err = strconv.ParseFloat(valueStr, 64)
			if err != nil {
				continue
			}
		}

		// Parse labels
		labels := make(map[string]string)
		if labelsStr != "" {
			labelMatches := labelRegex.FindAllStringSubmatch(labelsStr, -1)
			for _, lm := range labelMatches {
				labels[lm[1]] = lm[2]
			}
		}

		metric := Metric{
			Name:   name,
			Labels: labels,
			Value:  value,
			Help:   helpMap[name],
			Type:   typeMap[name],
		}

		// Group metrics by base name (strip _bucket, _count, _sum suffixes for histograms)
		baseName := name
		for _, suffix := range []string{"_bucket", "_count", "_sum", "_total"} {
			if strings.HasSuffix(name, suffix) {
				baseName = strings.TrimSuffix(name, suffix)
				break
			}
		}

		metrics[baseName] = append(metrics[baseName], metric)
		count++
	}

	return metrics, count
}

// fetchMetrics fetches metrics from the main server
func fetchMetrics() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://localhost:8080/metrics")
	if err != nil {
		return "", fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

func main() {
	// Initialize worker runtime
	runtime := worker.NewRuntime()

	// Initialize templates with file loader
	loader := func(name string) (string, error) {
		content, err := os.ReadFile(name)
		return string(content), err
	}
	tmpl = tqtemplate.NewTemplateWithLoader(loader)

	// Dashboard route - serves the HTML page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		data := map[string]interface{}{
			"PageTitle": "TQServer Metrics Dashboard",
			"Time":      time.Now().Format("2006-01-02 15:04:05"),
			"DevMode":   runtime.IsDevelopmentMode(),
		}

		output, err := tmpl.RenderFile("views/index.html", data)
		if err != nil {
			log.Printf("Template error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(output)))
		io.WriteString(w, output)
	})

	// API endpoint - returns parsed metrics as JSON
	http.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		rawMetrics, err := fetchMetrics()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		parsedMetrics, count := parsePrometheusMetrics(rawMetrics)

		response := MetricsResponse{
			Timestamp:   time.Now().Format(time.RFC3339),
			Metrics:     parsedMetrics,
			RawCount:    strings.Count(rawMetrics, "\n"),
			ParsedCount: count,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("JSON encode error: %v", err)
		}
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start server using runtime
	if err := runtime.StartServer(nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
