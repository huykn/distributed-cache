ids=(test-1763879237 test-1763879238 test-1763879239 test-1763879240 test-1763879241)
for id in "${ids[@]}"; do
    curl -X POST http://localhost:8080/create \
		-H "Content-Type: application/json" \
		-d "{\"id\":\"$$id\",\"title\":\"Test Post\",\"content\":\"This is a test post\",\"author\":\"makefile\"}" \
		| jq .
done
wrk -t3 -c200 -d10s --latency -H "accept: application/json" -s api.benchmark.random.lua http://127.0.0.1:9080