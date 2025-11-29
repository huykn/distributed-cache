ids = ("test-1763879237" "test-1763879238" "test-1763879239" "test-1763879240" "test-1763879241")
for id in $ids; do
    echo "Benchmarking $id..."
    curl -X POST http://localhost:8080/create \
		-H "Content-Type: application/json" \
		-d "{\"id\":\"$$POST_ID\",\"title\":\"Test Post\",\"content\":\"This is a test post\",\"author\":\"makefile\"}" \
		| jq .
done
wrk -t3 -c300 -d30s --latency  -H "accept: application/json" 'http://localhost:8081/post?id=test-1763879237'
