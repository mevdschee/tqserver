// Dashboard state
let availableMetrics = [];
let selectedMetrics = new Set();
let charts = {};

// Default metrics to show on first load
const DEFAULT_METRICS = [
    'tqserver_active_requests',
    'tqserver_process_goroutines',
    'tqserver_process_uptime_seconds',
    'tqserver_process_memory_bytes{type="heap"}'
];

// Chart colors palette
const COLORS = [
    '#3b82f6', // blue
    '#10b981', // green
    '#f59e0b', // amber
    '#ef4444', // red
    '#8b5cf6', // purple
    '#ec4899', // pink
    '#06b6d4', // cyan
    '#84cc16', // lime
];

// Format timestamp for display
function formatTime(timestamp) {
    const date = new Date(timestamp);
    return date.toLocaleTimeString();
}

// Format metric name for display (shorten labels)
function formatMetricName(name) {
    // Shorten long label values
    return name.replace(/\{([^}]+)\}/g, (match, labels) => {
        const shortened = labels.split(',').map(l => {
            const [key, value] = l.split('=');
            if (value && value.length > 20) {
                return `${key}=${value.substring(0, 17)}...`;
            }
            return l;
        }).join(', ');
        return `{${shortened}}`;
    });
}

// Fetch available metrics
async function fetchMetricsList() {
    try {
        const response = await fetch('api/metrics');
        const data = await response.json();
        availableMetrics = data.metrics || [];
        updateMetricsList();
        updateStatus(true);
    } catch (error) {
        console.error('Failed to fetch metrics list:', error);
        updateStatus(false);
    }
}

// Update the metrics list UI
function updateMetricsList() {
    const container = document.getElementById('metrics-list');
    const searchInput = document.getElementById('metric-search');
    const filter = searchInput.value.toLowerCase();

    // Filter metrics
    const filtered = availableMetrics.filter(m => m.toLowerCase().includes(filter));

    // Group metrics by prefix
    const groups = {};
    for (const metric of filtered) {
        const prefix = metric.split('{')[0].split('_').slice(0, 2).join('_');
        if (!groups[prefix]) groups[prefix] = [];
        groups[prefix].push(metric);
    }

    // Build HTML
    let html = '';
    for (const [prefix, metrics] of Object.entries(groups)) {
        html += `<div class="metric-group">`;
        html += `<div class="group-header">${prefix} (${metrics.length})</div>`;
        for (const metric of metrics) {
            const checked = selectedMetrics.has(metric) ? 'checked' : '';
            const displayName = formatMetricName(metric);
            html += `
                <label class="metric-item">
                    <input type="checkbox" value="${metric}" ${checked} onchange="toggleMetric('${metric}')">
                    <span title="${metric}">${displayName}</span>
                </label>
            `;
        }
        html += `</div>`;
    }

    container.innerHTML = html || '<div class="no-metrics">No metrics available yet. Waiting for data...</div>';
}

// Toggle metric selection
function toggleMetric(metricName) {
    if (selectedMetrics.has(metricName)) {
        selectedMetrics.delete(metricName);
        removeChart(metricName);
    } else {
        selectedMetrics.add(metricName);
        createChart(metricName);
    }
    saveSelectedMetrics();
}

// Create a chart for a metric
function createChart(metricName) {
    const container = document.getElementById('charts-container');

    // Create chart wrapper
    const wrapper = document.createElement('div');
    wrapper.className = 'chart-wrapper';
    wrapper.id = `chart-wrapper-${encodeURIComponent(metricName)}`;

    const colorIndex = Object.keys(charts).length % COLORS.length;

    wrapper.innerHTML = `
        <div class="chart-header">
            <h3>${formatMetricName(metricName)}</h3>
            <button class="remove-btn" onclick="toggleMetric('${metricName}')">×</button>
        </div>
        <div class="chart-container">
            <canvas id="chart-${encodeURIComponent(metricName)}"></canvas>
        </div>
        <div class="chart-stats" id="stats-${encodeURIComponent(metricName)}">
            <span>Current: --</span>
            <span>Min: --</span>
            <span>Max: --</span>
        </div>
    `;

    container.appendChild(wrapper);

    // Create Chart.js chart
    const ctx = document.getElementById(`chart-${encodeURIComponent(metricName)}`).getContext('2d');
    charts[metricName] = new Chart(ctx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: metricName,
                data: [],
                borderColor: COLORS[colorIndex],
                backgroundColor: COLORS[colorIndex] + '20',
                borderWidth: 2,
                fill: true,
                tension: 0.3,
                pointRadius: 0,
                pointHoverRadius: 4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: {
                duration: 0
            },
            interaction: {
                intersect: false,
                mode: 'index'
            },
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    titleColor: '#fff',
                    bodyColor: '#fff',
                    callbacks: {
                        label: function (context) {
                            return `Value: ${context.parsed.y.toLocaleString()}`;
                        }
                    }
                }
            },
            scales: {
                x: {
                    display: true,
                    grid: {
                        color: 'rgba(255, 255, 255, 0.1)'
                    },
                    ticks: {
                        color: '#888',
                        maxTicksLimit: 8
                    }
                },
                y: {
                    display: true,
                    grid: {
                        color: 'rgba(255, 255, 255, 0.1)'
                    },
                    ticks: {
                        color: '#888',
                        callback: function (value) {
                            if (value >= 1000000) return (value / 1000000).toFixed(1) + 'M';
                            if (value >= 1000) return (value / 1000).toFixed(1) + 'K';
                            return value.toFixed(value < 10 ? 2 : 0);
                        }
                    }
                }
            }
        }
    });

    // Fetch initial data for this chart
    fetchMetricData(metricName);
}

// Remove a chart
function removeChart(metricName) {
    const wrapper = document.getElementById(`chart-wrapper-${encodeURIComponent(metricName)}`);
    if (wrapper) {
        wrapper.remove();
    }
    if (charts[metricName]) {
        charts[metricName].destroy();
        delete charts[metricName];
    }
}

// Fetch data for a single metric
async function fetchMetricData(metricName) {
    try {
        const response = await fetch(`api/metrics/${encodeURIComponent(metricName)}`);
        if (!response.ok) return;

        const data = await response.json();
        updateChart(metricName, data.data);
    } catch (error) {
        console.error(`Failed to fetch data for ${metricName}:`, error);
    }
}

// Update chart with new data
function updateChart(metricName, dataPoints) {
    const chart = charts[metricName];
    if (!chart || !dataPoints) return;

    const labels = dataPoints.map(d => formatTime(d.timestamp));
    const values = dataPoints.map(d => d.value);

    chart.data.labels = labels;
    chart.data.datasets[0].data = values;
    chart.update('none');

    // Update stats
    if (values.length > 0) {
        const current = values[values.length - 1];
        const min = Math.min(...values);
        const max = Math.max(...values);

        const statsEl = document.getElementById(`stats-${encodeURIComponent(metricName)}`);
        if (statsEl) {
            statsEl.innerHTML = `
                <span>Current: <strong>${formatValue(current)}</strong></span>
                <span>Min: <strong>${formatValue(min)}</strong></span>
                <span>Max: <strong>${formatValue(max)}</strong></span>
            `;
        }
    }
}

// Format large numbers
function formatValue(value) {
    if (value >= 1000000) return (value / 1000000).toFixed(2) + 'M';
    if (value >= 1000) return (value / 1000).toFixed(2) + 'K';
    return value.toFixed(value < 10 ? 2 : 0);
}

// Update all charts
async function updateAllCharts() {
    for (const metricName of selectedMetrics) {
        await fetchMetricData(metricName);
    }
    document.getElementById('last-update').textContent = `Last update: ${new Date().toLocaleTimeString()}`;
}

// Update connection status
function updateStatus(connected) {
    const indicator = document.getElementById('status-indicator');
    const text = document.getElementById('status-text');

    if (connected) {
        indicator.className = 'indicator connected';
        text.textContent = `Connected (${availableMetrics.length} metrics)`;
    } else {
        indicator.className = 'indicator disconnected';
        text.textContent = 'Disconnected';
    }
}

// Save selected metrics to localStorage
function saveSelectedMetrics() {
    localStorage.setItem('selectedMetrics', JSON.stringify([...selectedMetrics]));
}

// Load selected metrics from localStorage
function loadSelectedMetrics() {
    const saved = localStorage.getItem('selectedMetrics');
    if (saved) {
        try {
            const metrics = JSON.parse(saved);
            metrics.forEach(m => selectedMetrics.add(m));
        } catch (e) {
            console.error('Failed to load saved metrics:', e);
        }
    }
}

// Initialize
async function init() {
    // Load saved selection
    loadSelectedMetrics();

    // Setup search filter
    const searchInput = document.getElementById('metric-search');
    searchInput.addEventListener('input', updateMetricsList);

    // Fetch initial metrics list
    await fetchMetricsList();

    // If no metrics selected, show defaults (if available)
    if (selectedMetrics.size === 0) {
        for (const metric of DEFAULT_METRICS) {
            if (availableMetrics.includes(metric)) {
                selectedMetrics.add(metric);
            }
        }
        // If no defaults match, select first 4 available
        if (selectedMetrics.size === 0 && availableMetrics.length > 0) {
            availableMetrics.slice(0, 4).forEach(m => selectedMetrics.add(m));
        }
        saveSelectedMetrics();
    }

    // Create charts for selected metrics
    for (const metric of selectedMetrics) {
        if (availableMetrics.includes(metric)) {
            createChart(metric);
        } else {
            // Metric no longer exists, remove from selection
            selectedMetrics.delete(metric);
        }
    }

    updateMetricsList();

    // Start auto-refresh
    setInterval(async () => {
        await fetchMetricsList();
        await updateAllCharts();
    }, 5000);
}

// Start when DOM is ready
document.addEventListener('DOMContentLoaded', init);
