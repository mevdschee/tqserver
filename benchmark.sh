#!/bin/bash
set -e

# Output file
results_file="benchmark_results.csv"
echo "Worker,Concurrency,RequestsPerSecond" > "$results_file"

# Check if ab is installed
if ! command -v ab &> /dev/null; then
    echo "Error: ab (Apache Benchmark) is not installed."
    echo "Please install it (e.g., sudo apt-get install apache2-utils) and try again."
    exit 1
fi

# Define workers and their URLs
# Maps worker name to URL
declare -A workers
workers["api"]="http://localhost:8080/api/bench"
workers["blog"]="http://localhost:8080/blog/bench.php"
workers["index"]="http://localhost:8080/bench"

# Duration for ab (seconds)
DURATION=5

echo "Starting benchmark..."
echo "Results will be written to $results_file"

# Loop through workers
for worker in "index" "api" "blog"; do
    url=${workers[$worker]}
    echo "------------------------------------------------"
    echo "Benchmarking Worker: $worker"
    echo "URL: $url"
    echo "------------------------------------------------"
    
    # Loop concurrency 1 to 10
    for con in {1..10}; do
        echo -n "  Concurrency $con... "
        
        # Run ab
        # -t: duration
        # -c: concurrency
        # 2>&1: capture stderr too just in case
        output=$(ab -t "$DURATION" -c "$con" "$url" 2>&1)
        
        # Check for failure
        if [[ $? -ne 0 ]]; then
            echo "Failed!"
            echo "$output"
            continue
        fi

        # Extract Requests per second
        # Example line: Requests per second:    16279.79 [#/sec] (mean)
        rps=$(echo "$output" | grep "Requests per second:" | awk '{print $4}')
        
        if [ -z "$rps" ]; then
            echo "Error parsing RPS."
            rps="0"
        fi
        
        echo "$rps req/sec"
        echo "$worker,$con,$rps" >> "$results_file"
    done
done

echo "------------------------------------------------"
echo "Benchmark complete."
