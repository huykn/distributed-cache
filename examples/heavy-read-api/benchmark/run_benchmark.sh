#!/bin/bash

# Benchmark Runner for Heavy-Read API
# Compares /post, /post-redis, and /post-marshal endpoints
# Tests both direct connection and via APISIX gateway

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPORT_FILE="${SCRIPT_DIR}/benchmark-report.md"
DIRECT_URL="http://localhost:8081"
APISIX_URL="http://localhost:9080"
WRITER_URL="http://localhost:8080"

# wrk configuration
WRK_THREADS="${WRK_THREADS:-4}"
WRK_CONNECTIONS="${WRK_CONNECTIONS:-400}"
WRK_DURATION="${WRK_DURATION:-10s}"

# Temp files for results
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."
    
    if ! command -v wrk &> /dev/null; then
        log_error "wrk is not installed. Please install it first."
        log_info "  macOS: brew install wrk"
        log_info "  Ubuntu: apt-get install wrk"
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        log_error "curl is not installed."
        exit 1
    fi
    
    log_success "All dependencies are available"
}

# Get system information
get_system_info() {
    log_info "Collecting system information..."
    
    # CPU info
    if [[ "$OSTYPE" == "darwin"* ]]; then
        CPU_MODEL=$(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo "Unknown")
        CPU_CORES=$(sysctl -n hw.ncpu 2>/dev/null || echo "Unknown")
        MEMORY=$(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1024 / 1024 / 1024 ))
        MEMORY="${MEMORY} GB"
    else
        CPU_MODEL=$(grep -m1 "model name" /proc/cpuinfo 2>/dev/null | cut -d: -f2 | xargs || echo "Unknown")
        CPU_CORES=$(nproc 2>/dev/null || echo "Unknown")
        MEMORY=$(free -h 2>/dev/null | grep Mem | awk '{print $2}' || echo "Unknown")
    fi
    
    # Go version
    GO_VERSION=$(go version 2>/dev/null | awk '{print $3}' || echo "Unknown")
    
    # Redis version (try to get from running container or local)
    REDIS_VERSION=$(redis-cli INFO server 2>/dev/null | grep redis_version | cut -d: -f2 | tr -d '\r' || \
                   docker exec $(docker ps -q --filter "name=redis" 2>/dev/null | head -1) redis-cli INFO server 2>/dev/null | grep redis_version | cut -d: -f2 | tr -d '\r' || \
                   echo "Unknown")
    
    # wrk version
    WRK_VERSION=$(wrk --version 2>&1 | head -1 || echo "Unknown")
}

# Create test posts
setup_test_data() {
    log_info "Creating test posts..."
    
    for i in 1 2 3 4 5; do
        curl -s -X POST "${WRITER_URL}/create" \
            -H "Content-Type: application/json" \
            -d "{\"id\":\"bench-post-$i\",\"title\":\"Benchmark Post $i\",\"content\":\"Content for benchmark test post $i with some additional text to make it realistic.\",\"author\":\"benchmark-runner\"}" \
            > /dev/null 2>&1 || true
    done
    
    # Wait for propagation
    sleep 2
    log_success "Test posts created"
}

# Run a single benchmark
run_benchmark() {
    local url=$1
    local script=$2
    local output_file=$3
    local label=$4
    
    log_info "Running benchmark: $label"
    
    wrk -t${WRK_THREADS} -c${WRK_CONNECTIONS} -d${WRK_DURATION} --latency \
        -s "${SCRIPT_DIR}/${script}" \
        "${url}" 2>&1 | tee "$output_file"
}

# Parse wrk output and extract metrics from our custom METRIC_ prefixed output
parse_wrk_output() {
    local file=$1

    # Extract metrics using grep for our custom METRIC_ prefixed lines
    local rps=$(grep "^METRIC_RPS:" "$file" | cut -d: -f2 | tr -d ' ')
    local transfer=$(grep "^METRIC_TRANSFER:" "$file" | cut -d: -f2 | tr -d ' ')
    local total_requests=$(grep "^METRIC_TOTAL_REQ:" "$file" | cut -d: -f2 | tr -d ' ')
    local total_errors=$(grep "^METRIC_ERRORS:" "$file" | cut -d: -f2 | tr -d ' ')
    local avg_latency=$(grep "^METRIC_AVG:" "$file" | cut -d: -f2 | tr -d ' ')
    local max_latency=$(grep "^METRIC_MAX:" "$file" | cut -d: -f2 | tr -d ' ')
    local stdev=$(grep "^METRIC_STDEV:" "$file" | cut -d: -f2 | tr -d ' ')
    local p50=$(grep "^METRIC_P50:" "$file" | cut -d: -f2 | tr -d ' ')
    local p90=$(grep "^METRIC_P90:" "$file" | cut -d: -f2 | tr -d ' ')
    local p99=$(grep "^METRIC_P99:" "$file" | cut -d: -f2 | tr -d ' ')

    echo "${rps}|${transfer}|${total_requests}|${total_errors}|${avg_latency}|${max_latency}|${stdev}|${p50}|${p90}|${p99}"
}

# Calculate improvement ratio
calc_improvement() {
    local base=$1
    local optimized=$2
    
    if [[ -z "$base" ]] || [[ -z "$optimized" ]] || [[ "$optimized" == "0" ]] || [[ "$optimized" == "0.00" ]]; then
        echo "N/A"
        return
    fi
    
    # Remove "ms" or "KB" suffix if present
    base=$(echo "$base" | sed 's/[^0-9.]//g')
    optimized=$(echo "$optimized" | sed 's/[^0-9.]//g')
    
    if [[ -z "$base" ]] || [[ -z "$optimized" ]] || [[ "$optimized" == "0" ]]; then
        echo "N/A"
        return
    fi
    
    local ratio=$(echo "scale=1; $base / $optimized" | bc 2>/dev/null || echo "N/A")
    echo "${ratio}x"
}

# Calculate throughput improvement (higher is better)
calc_throughput_improvement() {
    local base=$1
    local optimized=$2
    
    if [[ -z "$base" ]] || [[ -z "$optimized" ]] || [[ "$base" == "0" ]] || [[ "$base" == "0.00" ]]; then
        echo "N/A"
        return
    fi
    
    base=$(echo "$base" | sed 's/[^0-9.]//g')
    optimized=$(echo "$optimized" | sed 's/[^0-9.]//g')
    
    if [[ -z "$base" ]] || [[ -z "$optimized" ]] || [[ "$base" == "0" ]]; then
        echo "N/A"
        return
    fi
    
    local ratio=$(echo "scale=1; $optimized / $base" | bc 2>/dev/null || echo "N/A")
    echo "${ratio}x"
}

# Generate markdown report
generate_report() {
    log_info "Generating benchmark report..."

    # Parse all results
    IFS='|' read -r redis_rps redis_transfer redis_total redis_errors redis_avg redis_max redis_stdev redis_p50 redis_p90 redis_p99 <<< "$(parse_wrk_output "${TMP_DIR}/direct_redis.txt")"
    IFS='|' read -r marshal_rps marshal_transfer marshal_total marshal_errors marshal_avg marshal_max marshal_stdev marshal_p50 marshal_p90 marshal_p99 <<< "$(parse_wrk_output "${TMP_DIR}/direct_marshal.txt")"
    IFS='|' read -r post_rps post_transfer post_total post_errors post_avg post_max post_stdev post_p50 post_p90 post_p99 <<< "$(parse_wrk_output "${TMP_DIR}/direct_post.txt")"

    IFS='|' read -r ax_redis_rps ax_redis_transfer ax_redis_total ax_redis_errors ax_redis_avg ax_redis_max ax_redis_stdev ax_redis_p50 ax_redis_p90 ax_redis_p99 <<< "$(parse_wrk_output "${TMP_DIR}/apisix_redis.txt")"
    IFS='|' read -r ax_marshal_rps ax_marshal_transfer ax_marshal_total ax_marshal_errors ax_marshal_avg ax_marshal_max ax_marshal_stdev ax_marshal_p50 ax_marshal_p90 ax_marshal_p99 <<< "$(parse_wrk_output "${TMP_DIR}/apisix_marshal.txt")"
    IFS='|' read -r ax_post_rps ax_post_transfer ax_post_total ax_post_errors ax_post_avg ax_post_max ax_post_stdev ax_post_p50 ax_post_p90 ax_post_p99 <<< "$(parse_wrk_output "${TMP_DIR}/apisix_post.txt")"

    # Calculate improvements for direct connection
    local p50_vs_redis=$(calc_improvement "$redis_p50" "$post_p50")
    local p50_vs_marshal=$(calc_improvement "$marshal_p50" "$post_p50")
    local p99_vs_redis=$(calc_improvement "$redis_p99" "$post_p99")
    local p99_vs_marshal=$(calc_improvement "$marshal_p99" "$post_p99")
    local rps_vs_redis=$(calc_throughput_improvement "$redis_rps" "$post_rps")
    local rps_vs_marshal=$(calc_throughput_improvement "$marshal_rps" "$post_rps")

    # Calculate improvements for APISIX
    local ax_p50_vs_redis=$(calc_improvement "$ax_redis_p50" "$ax_post_p50")
    local ax_p50_vs_marshal=$(calc_improvement "$ax_marshal_p50" "$ax_post_p50")
    local ax_p99_vs_redis=$(calc_improvement "$ax_redis_p99" "$ax_post_p99")
    local ax_p99_vs_marshal=$(calc_improvement "$ax_marshal_p99" "$ax_post_p99")
    local ax_rps_vs_redis=$(calc_throughput_improvement "$ax_redis_rps" "$ax_post_rps")
    local ax_rps_vs_marshal=$(calc_throughput_improvement "$ax_marshal_rps" "$ax_post_rps")

    # Write report
    cat > "$REPORT_FILE" << EOF
# Heavy-Read API Benchmark Report

**Generated:** $(date -u +"%Y-%m-%d %H:%M:%S UTC")

## Hardware & Configuration

| Component | Details |
|-----------|---------|
| **CPU Model** | ${CPU_MODEL} |
| **CPU Cores** | ${CPU_CORES} |
| **Memory (RAM)** | ${MEMORY} |
| **Go Version** | ${GO_VERSION} |
| **Redis Version** | ${REDIS_VERSION} |
| **wrk Version** | ${WRK_VERSION} |

### wrk Configuration

| Parameter | Value |
|-----------|-------|
| Threads | ${WRK_THREADS} |
| Connections | ${WRK_CONNECTIONS} |
| Duration | ${WRK_DURATION} |

---

## Direct Connection (${DIRECT_URL})

### Latency Comparison

| Metric | /post-redis | /post-marshal | /post (Optimized) | Improvement vs Redis | Improvement vs Marshal |
|--------|-------------|---------------|-------------------|---------------------|------------------------|
| **P50 Latency** | ${redis_p50} ms | ${marshal_p50} ms | ${post_p50} ms | ${p50_vs_redis} faster | ${p50_vs_marshal} faster |
| **P90 Latency** | ${redis_p90} ms | ${marshal_p90} ms | ${post_p90} ms | $(calc_improvement "$redis_p90" "$post_p90") faster | $(calc_improvement "$marshal_p90" "$post_p90") faster |
| **P99 Latency** | ${redis_p99} ms | ${marshal_p99} ms | ${post_p99} ms | ${p99_vs_redis} faster | ${p99_vs_marshal} faster |
| **Avg Latency** | ${redis_avg} ms | ${marshal_avg} ms | ${post_avg} ms | $(calc_improvement "$redis_avg" "$post_avg") faster | $(calc_improvement "$marshal_avg" "$post_avg") faster |
| **Max Latency** | ${redis_max} ms | ${marshal_max} ms | ${post_max} ms | $(calc_improvement "$redis_max" "$post_max") faster | $(calc_improvement "$marshal_max" "$post_max") faster |
| **Stdev** | ${redis_stdev} ms | ${marshal_stdev} ms | ${post_stdev} ms | - | - |

### Throughput Comparison

| Metric | /post-redis | /post-marshal | /post (Optimized) | Improvement vs Redis | Improvement vs Marshal |
|--------|-------------|---------------|-------------------|---------------------|------------------------|
| **Requests/sec** | ${redis_rps} | ${marshal_rps} | ${post_rps} | ${rps_vs_redis} higher | ${rps_vs_marshal} higher |
| **Transfer/sec** | ${redis_transfer} KB | ${marshal_transfer} KB | ${post_transfer} KB | - | - |
| **Total Requests** | ${redis_total} | ${marshal_total} | ${post_total} | - | - |
| **Total Errors** | ${redis_errors} | ${marshal_errors} | ${post_errors} | - | - |

---

## Via APISIX Gateway (${APISIX_URL})

### Latency Comparison

| Metric | /post-redis | /post-marshal | /post (Optimized) | Improvement vs Redis | Improvement vs Marshal |
|--------|-------------|---------------|-------------------|---------------------|------------------------|
| **P50 Latency** | ${ax_redis_p50} ms | ${ax_marshal_p50} ms | ${ax_post_p50} ms | ${ax_p50_vs_redis} faster | ${ax_p50_vs_marshal} faster |
| **P90 Latency** | ${ax_redis_p90} ms | ${ax_marshal_p90} ms | ${ax_post_p90} ms | $(calc_improvement "$ax_redis_p90" "$ax_post_p90") faster | $(calc_improvement "$ax_marshal_p90" "$ax_post_p90") faster |
| **P99 Latency** | ${ax_redis_p99} ms | ${ax_marshal_p99} ms | ${ax_post_p99} ms | ${ax_p99_vs_redis} faster | ${ax_p99_vs_marshal} faster |
| **Avg Latency** | ${ax_redis_avg} ms | ${ax_marshal_avg} ms | ${ax_post_avg} ms | $(calc_improvement "$ax_redis_avg" "$ax_post_avg") faster | $(calc_improvement "$ax_marshal_avg" "$ax_post_avg") faster |
| **Max Latency** | ${ax_redis_max} ms | ${ax_marshal_max} ms | ${ax_post_max} ms | $(calc_improvement "$ax_redis_max" "$ax_post_max") faster | $(calc_improvement "$ax_marshal_max" "$ax_post_max") faster |
| **Stdev** | ${ax_redis_stdev} ms | ${ax_marshal_stdev} ms | ${ax_post_stdev} ms | - | - |

### Throughput Comparison

| Metric | /post-redis | /post-marshal | /post (Optimized) | Improvement vs Redis | Improvement vs Marshal |
|--------|-------------|---------------|-------------------|---------------------|------------------------|
| **Requests/sec** | ${ax_redis_rps} | ${ax_marshal_rps} | ${ax_post_rps} | ${ax_rps_vs_redis} higher | ${ax_rps_vs_marshal} higher |
| **Transfer/sec** | ${ax_redis_transfer} KB | ${ax_marshal_transfer} KB | ${ax_post_transfer} KB | - | - |
| **Total Requests** | ${ax_redis_total} | ${ax_marshal_total} | ${ax_post_total} | - | - |
| **Total Errors** | ${ax_redis_errors} | ${ax_marshal_errors} | ${ax_post_errors} | - | - |

---

## Endpoint Descriptions

| Endpoint | Description |
|----------|-------------|
| **/post** | Optimized endpoint using \`OnSetLocalCache\` callback with pre-processed \`CachedPost\` wrapper (zero CPU-bound operations on read path) |
| **/post-redis** | Endpoint that fetches directly from Redis on every request (bypasses local cache) |
| **/post-marshal** | Endpoint that uses local cache but re-marshals the cached data to JSON on every request |

## Key Insights

1. **Local Cache Advantage**: The optimized \`/post\` endpoint eliminates all serialization overhead by storing pre-processed data in local cache
2. **Redis Latency**: Direct Redis fetches add significant network latency compared to local cache hits
3. **Marshal Overhead**: JSON marshalling on every request adds CPU overhead even with local cache
4. **APISIX Overhead**: The gateway adds consistent latency overhead but preserves relative performance differences

---

*Report generated by run_benchmark.sh*
EOF

    log_success "Report saved to: ${REPORT_FILE}"
}

# Main execution
main() {
    echo ""
    echo "=========================================="
    echo "  Heavy-Read API Benchmark Runner"
    echo "=========================================="
    echo ""

    check_dependencies
    get_system_info
    setup_test_data

    echo ""
    log_info "Starting benchmarks (this may take several minutes)..."
    echo ""

    # Run Direct Connection benchmarks
    echo "========================================"
    echo "  Direct Connection Benchmarks"
    echo "========================================"

    run_benchmark "$DIRECT_URL" "benchmark_post_redis.lua" "${TMP_DIR}/direct_redis.txt" "Direct /post-redis"
    sleep 2

    run_benchmark "$DIRECT_URL" "benchmark_post_marshal.lua" "${TMP_DIR}/direct_marshal.txt" "Direct /post-marshal"
    sleep 2

    run_benchmark "$DIRECT_URL" "benchmark_post.lua" "${TMP_DIR}/direct_post.txt" "Direct /post (Optimized)"
    sleep 2

    # Run APISIX benchmarks
    echo ""
    echo "========================================"
    echo "  APISIX Gateway Benchmarks"
    echo "========================================"

    run_benchmark "$APISIX_URL" "benchmark_post_redis.lua" "${TMP_DIR}/apisix_redis.txt" "APISIX /post-redis"
    sleep 2

    run_benchmark "$APISIX_URL" "benchmark_post_marshal.lua" "${TMP_DIR}/apisix_marshal.txt" "APISIX /post-marshal"
    sleep 2

    run_benchmark "$APISIX_URL" "benchmark_post.lua" "${TMP_DIR}/apisix_post.txt" "APISIX /post (Optimized)"

    echo ""
    echo "========================================"
    echo "  Generating Report"
    echo "========================================"

    generate_report

    echo ""
    log_success "Benchmark complete!"
    echo ""
    echo "View the report: cat ${REPORT_FILE}"
    echo ""
}

# Run main function
main "$@"
