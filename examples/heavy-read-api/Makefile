.PHONY: help start stop restart logs benchmark clean test-write test-read

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

start: ## Start all services
	docker-compose up -d --build
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Services started! Access points:"
	@echo "  - Gateway:    http://localhost:9080"
	@echo "  - Writer:     http://localhost:8080"
	@echo "  - Reader:     http://localhost:8081"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - Grafana:    http://localhost:3000 (admin/admin)"

stop: ## Stop all services
	docker-compose down

restart: ## Restart all services
	docker-compose restart

logs: ## Show logs from all services
	docker-compose logs -f

logs-writer: ## Show writer logs
	docker-compose logs -f writer

logs-reader: ## Show reader logs
	docker-compose logs -f reader

logs-redis: ## Show Redis logs
	docker-compose logs -f redis

benchmark: ## Run performance benchmark
	@echo "Running benchmark..."
	@./benchmark/run_benchmark.sh

test-write: ## Create a test post
	$(eval POST_ID=test-$(shell date +%s))
	@echo "Creating test post $(POST_ID)..."
	@curl -X POST http://localhost:8080/create \
		-H "Content-Type: application/json" \
		-d '{"id":"$(POST_ID)","title":"Test Post","content":"This is a test post","author":"makefile"}' \
		| jq .

test-read: ## Read a test post (requires POST_ID env var)
	@if [ -z "$(POST_ID)" ]; then \
		echo "Error: POST_ID not set. Usage: make test-read POST_ID=post-1"; \
		exit 1; \
	fi
	@echo "Reading post $(POST_ID)..."
	@curl -s http://localhost:9080/post?id=$(POST_ID) | jq .

test: ## Run integration test (create and read a post)
	@echo "Running integration test..."
	@POST_ID=test-$$(date +%s); \
	echo "Creating test post $$POST_ID..."; \
	curl -X POST http://localhost:8080/create \
		-H "Content-Type: application/json" \
		-d "{\"id\":\"$$POST_ID\",\"title\":\"Test Post\",\"content\":\"This is a test post\",\"author\":\"makefile\"}" \
		| jq .; \
	echo "Reading post $$POST_ID via gateway..."; \
	curl -s "http://localhost:80/post?id=$$POST_ID" | jq .; \
	echo "Reading post $$POST_ID via direct reader..."; \
	curl -s "http://localhost:8081/post?id=$$POST_ID" | jq .; \
	echo "Tests completed!"

health: ## Check health of all services
	@echo "Checking service health..."
	@echo -n "Gateway: "
	@curl -s http://localhost:9080/health | jq -r .status
	@echo -n "Writer:  "
	@curl -s http://localhost:8080/health | jq -r .status
	@echo -n "Reader:  "
	@curl -s http://localhost:8081/health | jq -r .status

clean: ## Stop services and remove volumes
	docker-compose down -v
	@echo "Cleaned up all services and volumes"

build: ## Build Docker images
	docker-compose build

scale-readers: ## Scale reader instances (usage: make scale-readers N=5)
	@if [ -z "$(N)" ]; then \
		echo "Error: N not set. Usage: make scale-readers N=5"; \
		exit 1; \
	fi
	docker-compose up -d --scale reader=$(N)
	@echo "Scaled readers to $(N) instances"

stats: ## Show cache statistics
	@echo "Reader cache stats:"
	@curl -s http://localhost:8081/metrics | jq .
