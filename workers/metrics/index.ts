import { serve } from "bun";
import { readFileSync, existsSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const port = parseInt(process.env.PORT || "3000");
const workerId = process.env.WORKER_PORT || "unknown";

// Configuration
const SCRAPE_INTERVAL_MS = 5000; // 5 seconds
const RETENTION_MINUTES = 60;
const MAX_DATA_POINTS = (RETENTION_MINUTES * 60 * 1000) / SCRAPE_INTERVAL_MS; // 720 points

// Get the server port from environment or default
const SERVER_PORT = process.env.TQSERVER_PORT || "8080";
const METRICS_URL = `http://localhost:${SERVER_PORT}/metrics`;

// Time series storage: metric name -> array of {timestamp, value}
interface DataPoint {
    timestamp: number;
    value: number;
}

const timeSeries: Map<string, DataPoint[]> = new Map();

// Parse Prometheus metrics format
function parsePrometheusMetrics(text: string): Map<string, number> {
    const metrics = new Map<string, number>();
    const lines = text.split("\n");

    for (const line of lines) {
        // Skip comments and empty lines
        if (line.startsWith("#") || line.trim() === "") continue;

        // Parse metric line: metric_name{labels} value
        // or: metric_name value
        const match = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*(?:\{[^}]*\})?)[\s]+([0-9.eE+-]+)$/);
        if (match) {
            const metricName = match[1];
            const value = parseFloat(match[2]);
            if (!isNaN(value)) {
                metrics.set(metricName, value);
            }
        }
    }

    return metrics;
}

// Scrape metrics from the Prometheus endpoint
async function scrapeMetrics() {
    try {
        const response = await fetch(METRICS_URL);
        if (!response.ok) {
            console.error(`[Metrics] Failed to scrape: ${response.status}`);
            return;
        }

        const text = await response.text();
        const metrics = parsePrometheusMetrics(text);
        const now = Date.now();

        // Store each metric value
        for (const [name, value] of metrics) {
            let series = timeSeries.get(name);
            if (!series) {
                series = [];
                timeSeries.set(name, series);
            }

            // Add new data point
            series.push({ timestamp: now, value });

            // Remove old data points (keep last 60 minutes)
            const cutoff = now - RETENTION_MINUTES * 60 * 1000;
            while (series.length > 0 && series[0].timestamp < cutoff) {
                series.shift();
            }

            // Also enforce max data points limit
            while (series.length > MAX_DATA_POINTS) {
                series.shift();
            }
        }

        console.log(`[Metrics] Scraped ${metrics.size} metrics, storing ${timeSeries.size} series`);
    } catch (error) {
        console.error(`[Metrics] Scrape error:`, error);
    }
}

// Start scraping
setInterval(scrapeMetrics, SCRAPE_INTERVAL_MS);
scrapeMetrics(); // Initial scrape

// Read static files
function readStaticFile(filePath: string): string | null {
    const fullPath = join(__dirname, filePath);
    if (existsSync(fullPath)) {
        return readFileSync(fullPath, "utf8");
    }
    return null;
}

// Get content type for file
function getContentType(path: string): string {
    if (path.endsWith(".html")) return "text/html";
    if (path.endsWith(".css")) return "text/css";
    if (path.endsWith(".js")) return "application/javascript";
    return "text/plain";
}

// HTTP server
serve({
    port,
    fetch(req) {
        const url = new URL(req.url);
        const path = url.pathname;

        console.log(`[Worker ${workerId}] ${req.method} ${path}`);

        // Health check
        if (path === "/health") {
            return Response.json({ status: "ok", worker: workerId, metricsCount: timeSeries.size });
        }

        // API: List all available metrics
        if (path === "/api/metrics") {
            const metricsList = Array.from(timeSeries.keys()).sort();
            return Response.json({ metrics: metricsList, count: metricsList.length });
        }

        // API: Get time series data for a specific metric
        if (path.startsWith("/api/metrics/")) {
            const metricName = decodeURIComponent(path.substring("/api/metrics/".length));
            const series = timeSeries.get(metricName);
            if (series) {
                return Response.json({ name: metricName, data: series });
            }
            return Response.json({ error: "Metric not found" }, { status: 404 });
        }

        // API: Get multiple metrics at once (for dashboard)
        if (path === "/api/dashboard") {
            const metricsParam = url.searchParams.get("metrics");
            if (!metricsParam) {
                return Response.json({ error: "Missing metrics parameter" }, { status: 400 });
            }

            const requestedMetrics = metricsParam.split(",").map(m => decodeURIComponent(m.trim()));
            const result: Record<string, DataPoint[]> = {};

            for (const name of requestedMetrics) {
                const series = timeSeries.get(name);
                if (series) {
                    result[name] = series;
                }
            }

            return Response.json({ data: result });
        }

        // Static files
        if (path.startsWith("/public/")) {
            const content = readStaticFile(path);
            if (content) {
                return new Response(content, {
                    headers: { "Content-Type": getContentType(path) }
                });
            }
        }

        // Dashboard HTML (root path)
        if (path === "/" || path === "") {
            const html = readStaticFile("views/index.html");
            if (html) {
                return new Response(html, {
                    headers: { "Content-Type": "text/html" }
                });
            }
        }

        // 404
        return Response.json({ error: "Not found" }, { status: 404 });
    },
});

console.log(`Metrics Worker listening on port ${port}`);
