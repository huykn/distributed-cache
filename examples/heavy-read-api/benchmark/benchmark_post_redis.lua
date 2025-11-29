-- Benchmark script for /post-redis endpoint (direct Redis fetch, bypasses local cache)
-- Usage: wrk -t4 -c100 -d30s --latency -s benchmark_post_redis.lua <url>

-- Test post IDs (created by setup script)
local post_ids = {"bench-post-1", "bench-post-2", "bench-post-3", "bench-post-4", "bench-post-5"}

-- Request counter for round-robin
local counter = 0

function request()
    counter = counter + 1
    local id = post_ids[(counter % #post_ids) + 1]
    if counter == 5 then
        counter = 0
    end
    return wrk.format("GET", "/post-redis?id=" .. id)
end

function done(summary, latency, requests)
    io.write("\n")
    io.write("=== BENCHMARK RESULTS ===\n")
    io.write("ENDPOINT: /post-redis\n")
    io.write(string.format("METRIC_RPS: %.2f\n", summary.requests / (summary.duration / 1000000)))
    io.write(string.format("METRIC_TRANSFER: %.2f\n", (summary.bytes / (summary.duration / 1000000)) / 1024))
    io.write(string.format("METRIC_TOTAL_REQ: %d\n", summary.requests))
    io.write(string.format("METRIC_ERRORS: %d\n", summary.errors.connect + summary.errors.read + summary.errors.write + summary.errors.status + summary.errors.timeout))
    io.write(string.format("METRIC_AVG: %.2f\n", latency.mean / 1000))
    io.write(string.format("METRIC_MAX: %.2f\n", latency.max / 1000))
    io.write(string.format("METRIC_STDEV: %.2f\n", latency.stdev / 1000))
    io.write(string.format("METRIC_P50: %.2f\n", latency:percentile(50) / 1000))
    io.write(string.format("METRIC_P90: %.2f\n", latency:percentile(90) / 1000))
    io.write(string.format("METRIC_P99: %.2f\n", latency:percentile(99) / 1000))
    io.write("=== END RESULTS ===\n")
end
