module heavy-write-api-poc

go 1.25

require (
	github.com/go-sql-driver/mysql v1.8.1
	github.com/huykn/distributed-cache v0.0.0
	github.com/redis/go-redis/v9 v9.17.3
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgraph-io/ristretto v0.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
)

replace github.com/huykn/distributed-cache => ../../../
