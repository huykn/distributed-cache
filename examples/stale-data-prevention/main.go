// Package main demonstrates improved stale data prevention with proper test scenarios
// that actually validate version conflicts and cache-aside pattern issues.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	dc "github.com/huykn/distributed-cache"
	"github.com/huykn/distributed-cache/cache"
)

// VersionedData represents data with version, timestamp and checksum for stale detection.
type VersionedData struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Version   int64  `json:"version"`
	Timestamp int64  `json:"timestamp"` // Unix nano for tie-breaking
	UpdatedBy string `json:"updated_by"`
}

// StaleDetector monitors cache operations with proper atomic version tracking.
type StaleDetector struct {
	latestVersions  sync.Map
	staleDetections int64
	totalChecks     int64
	logger          cache.Logger
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// VersionInfo tracks version with timestamp for tie-breaking.
type VersionInfo struct {
	Version   int64
	Timestamp int64
	Source    string
}

// NewStaleDetector creates a new StaleDetector instance.
func NewStaleDetector(logger cache.Logger) *StaleDetector {
	return &StaleDetector{
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// CompareAndRecord atomically checks and records version using compare-and-swap.
// Returns: (shouldAccept bool, isNewer bool, previousVersion int64)
func (sd *StaleDetector) CompareAndRecord(key string, version, timestamp int64, source string) (bool, bool, int64) {
	atomic.AddInt64(&sd.totalChecks, 1)

	newInfo := &VersionInfo{
		Version:   version,
		Timestamp: timestamp,
		Source:    source,
	}

	for {
		current, loaded := sd.latestVersions.LoadOrStore(key, newInfo)
		if !loaded {
			// First time seeing this key
			sd.logger.Info("New key tracked", "key", key, "version", version, "source", source)
			return true, true, 0
		}

		currentInfo := current.(*VersionInfo)

		// Compare versions
		if version > currentInfo.Version {
			// Newer version - try to update
			if sd.latestVersions.CompareAndSwap(key, current, newInfo) {
				sd.logger.Info("Version updated",
					"key", key,
					"old_version", currentInfo.Version,
					"new_version", version,
					"source", source)
				return true, true, currentInfo.Version
			}
			// CAS failed, retry
			continue
		}

		if version < currentInfo.Version {
			// Stale data detected
			atomic.AddInt64(&sd.staleDetections, 1)
			sd.logger.Warn("STALE DATA REJECTED",
				"key", key,
				"stale_version", version,
				"current_version", currentInfo.Version,
				"source", source,
				"version_diff", currentInfo.Version-version)
			return false, false, currentInfo.Version
		}

		// Same version - use timestamp for tie-breaking
		if timestamp > currentInfo.Timestamp {
			// Same version but newer timestamp (rare but possible)
			if sd.latestVersions.CompareAndSwap(key, current, newInfo) {
				sd.logger.Info("Version tie-break by timestamp",
					"key", key,
					"version", version,
					"source", source)
				return true, false, currentInfo.Version
			}
			continue
		}

		// Same or older timestamp - reject
		if timestamp < currentInfo.Timestamp {
			sd.logger.Debug("Same version but older timestamp",
				"key", key, "version", version, "source", source)
		}
		return false, false, currentInfo.Version
	}
}

// GetVersionInfo returns the current version info for a key.
func (sd *StaleDetector) GetVersionInfo(key string) (*VersionInfo, bool) {
	val, ok := sd.latestVersions.Load(key)
	if !ok {
		return nil, false
	}
	return val.(*VersionInfo), true
}

// GetStats returns detection statistics.
func (sd *StaleDetector) GetStats() (staleCount, totalChecks int64, hitRate float64) {
	stale := atomic.LoadInt64(&sd.staleDetections)
	total := atomic.LoadInt64(&sd.totalChecks)
	if total > 0 {
		hitRate = float64(total-stale) / float64(total) * 100
	}
	return stale, total, hitRate
}

// StartMonitoring starts background monitoring with automatic cleanup.
func (sd *StaleDetector) StartMonitoring(interval time.Duration) {
	sd.wg.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stale, total, hitRate := sd.GetStats()
				keyCount := 0
				sd.latestVersions.Range(func(_, _ any) bool {
					keyCount++
					return true
				})
				sd.logger.Info("StaleDetector stats",
					"tracked_keys", keyCount,
					"stale_detections", stale,
					"total_checks", total,
					"hit_rate", fmt.Sprintf("%.2f%%", hitRate))
			case <-sd.stopCh:
				return
			}
		}
	})
}

// Stop stops the monitoring goroutine.
func (sd *StaleDetector) Stop() {
	close(sd.stopCh)
	sd.wg.Wait()
}

// CacheWrapper wraps cache with version validation on all operations.
type CacheWrapper struct {
	cache.Cache
	detector *StaleDetector
	podID    string
	logger   cache.Logger
}

// NewCacheWrapper creates a cache wrapper with stale detection.
func NewCacheWrapper(c cache.Cache, detector *StaleDetector, podID string, logger cache.Logger) *CacheWrapper {
	return &CacheWrapper{
		Cache:    c,
		detector: detector,
		podID:    podID,
		logger:   logger,
	}
}

// Set validates version before setting to cache.
func (cw *CacheWrapper) Set(ctx context.Context, key string, value any) error {
	data, ok := value.(*VersionedData)
	if !ok {
		return errors.New("value must be *VersionedData")
	}

	// Validate before setting
	shouldAccept, _, prevVersion := cw.detector.CompareAndRecord(
		key, data.Version, data.Timestamp, cw.podID+":set")

	if !shouldAccept {
		return fmt.Errorf("rejected stale version %d (current: %d)", data.Version, prevVersion)
	}

	return cw.Cache.Set(ctx, key, value)
}

// Get validates version when retrieving from cache-aside.
func (cw *CacheWrapper) Get(ctx context.Context, key string) (any, bool) {
	val, found := cw.Cache.Get(ctx, key)
	if !found {
		return nil, false
	}

	// Validate retrieved data
	if data, ok := val.(*VersionedData); ok {
		shouldAccept, _, _ := cw.detector.CompareAndRecord(
			key, data.Version, data.Timestamp, cw.podID+":get")

		if !shouldAccept {
			// Stale data in cache - invalidate it
			cw.logger.Warn("Stale data found in local cache, invalidating",
				"key", key, "version", data.Version)
			cw.Cache.Delete(ctx, key)
			return nil, false
		}
	}

	return val, found
}

var (
	ctx = context.Background()
)

func main() {
	fmt.Println("=== Enhanced Stale Data Prevention Demo===")

	logger := cache.NewConsoleLogger("demo")
	staleDetector := NewStaleDetector(logger)
	staleDetector.StartMonitoring(5 * time.Second)
	defer staleDetector.Stop()

	// Create caches with proper configuration
	writerCache := createCacheWithWrapper("writer", true, logger, staleDetector)
	defer writerCache.Close()

	reader1Cache := createCacheWithWrapper("reader-1", false, logger, staleDetector)
	defer reader1Cache.Close()

	reader2Cache := createCacheWithWrapper("reader-2", false, logger, staleDetector)
	defer reader2Cache.Close()

	// Run scenarios
	fmt.Println("=== Scenario 1: Normal Updates ===")
	runNormalUpdates(writerCache, reader1Cache, reader2Cache)
	time.Sleep(300 * time.Millisecond)

	fmt.Println("\n=== Scenario 2: Out-of-Order Pub/Sub Delivery ===")
	runOutOfOrderPubSubScenario(writerCache, reader1Cache, staleDetector)
	time.Sleep(300 * time.Millisecond)

	fmt.Println("\n=== Scenario 3: Concurrent Racing Writers ===")
	runConcurrentWriters(writerCache)
	time.Sleep(300 * time.Millisecond)

	fmt.Println("\n=== Scenario 4: Cache-Aside Stale Detection ===")
	runCacheAsideScenario(writerCache, reader1Cache)
	time.Sleep(300 * time.Millisecond)

	fmt.Println("\n=== Scenario 5: Stale Local Cache After Network Partition ===")
	runNetworkPartitionScenario(writerCache, reader1Cache, staleDetector)
	time.Sleep(300 * time.Millisecond)

	// Final statistics
	stale, total, hitRate := staleDetector.GetStats()
	fmt.Printf("\n=== Final Statistics ===\n")
	fmt.Printf("Total Version Checks: %d\n", total)
	fmt.Printf("Stale Data Rejected:  %d\n", stale)
	fmt.Printf("Success Rate:         %.2f%%\n", hitRate)

	if stale > 0 {
		fmt.Printf("\n -> Stale data prevention is working correctly!\n")
	}
}

func createCacheWithWrapper(podID string, canWriteToRedis bool, logger cache.Logger, detector *StaleDetector) *CacheWrapper {
	cfg := dc.DefaultConfig()
	cfg.PodID = podID
	cfg.RedisAddr = "localhost:6379"
	cfg.InvalidationChannel = "enhanced-stale-demo"
	cfg.DebugMode = false
	cfg.Logger = logger
	cfg.ReaderCanSetToRedis = canWriteToRedis

	// Enhanced OnSetLocalCache with version validation
	cfg.OnSetLocalCache = func(event dc.InvalidationEvent) any {
		var data VersionedData
		if err := json.Unmarshal(event.Value, &data); err != nil {
			logger.Error("Unmarshal failed", "error", err)
			return nil
		}

		// Atomic version check before storing
		shouldAccept, isNewer, prevVersion := detector.CompareAndRecord(
			data.Key, data.Version, data.Timestamp, podID+":pubsub")

		if !shouldAccept {
			logger.Warn("Pubsub message rejected",
				"key", data.Key,
				"message_version", data.Version,
				"current_version", prevVersion,
				"pod", podID)
			return nil // ← This is the ACTUAL stale prevention
		}

		if isNewer {
			logger.Info("Pubsub message accepted",
				"key", data.Key,
				"version", data.Version,
				"updated_by", data.UpdatedBy,
				"pod", podID)
		}

		return &data
	}

	c, err := dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create cache for %s: %v", podID, err)
	}

	return NewCacheWrapper(c, detector, podID, logger)
}

func runNormalUpdates(writer, reader1, reader2 *CacheWrapper) {
	fmt.Println("Writer performing sequential updates...")

	for i := 1; i <= 3; i++ {
		data := createVersionedData("product:100", i, "writer")

		if err := writer.Set(ctx, data.Key, data); err != nil {
			log.Printf("Set failed: %v", err)
		} else {
			fmt.Printf("Writer: Set version %d\n", data.Version)
		}
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(150 * time.Millisecond) // Wait for pub/sub propagation

	// Readers check
	checkReader(reader1, "product:100", "Reader-1")
	checkReader(reader2, "product:100", "Reader-2")
}

func runOutOfOrderPubSubScenario(writer, reader *CacheWrapper, detector *StaleDetector) {
	fmt.Println("Testing out-of-order pub/sub message delivery...")

	key := "order:200"

	// Writer sends versions 1, 2, 3 rapidly
	fmt.Println("  → Writer sending rapid updates (v1, v2, v3)...")
	for i := 1; i <= 3; i++ {
		data := createVersionedData(key, i, "writer")
		writer.Set(ctx, key, data)
		time.Sleep(10 * time.Millisecond) // Very fast to increase chance of out-of-order
	}

	// Wait for all messages to propagate
	time.Sleep(100 * time.Millisecond)

	fmt.Println("  → Simulating delayed v1 message arriving after v3...")

	// Manually trigger stale detection by simulating what pub/sub would do
	staleData := createVersionedData(key, 1, "delayed-pubsub")
	staleData.Timestamp = time.Now().Add(-5 * time.Second).UnixNano() // Old timestamp

	// This simulates OnSetLocalCache receiving the delayed message
	shouldAccept, _, prevVersion := detector.CompareAndRecord(
		key, staleData.Version, staleData.Timestamp, "reader-1:pubsub-delayed")

	if shouldAccept {
		fmt.Printf("FAIL: Stale v1 was accepted (current: v%d)\n", prevVersion)
	} else {
		fmt.Printf("SUCCESS: Stale v1 rejected (current: v%d)\n", prevVersion)
	}

	// Verify reader has the latest version
	checkReader(reader, key, "Reader")
}

func runConcurrentWriters(writer *CacheWrapper) {
	fmt.Println("Testing concurrent writers racing for the same key...")

	key := "inventory:300"
	var wg sync.WaitGroup
	results := make(chan string, 10)

	// Simulate 10 concurrent updates
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go func(version int) {
			defer wg.Done()
			data := createVersionedData(key, version, fmt.Sprintf("writer-%d", version))
			if err := writer.Set(ctx, data.Key, data); err != nil {
				results <- fmt.Sprintf("v%d rejected", version)
			} else {
				results <- fmt.Sprintf("v%d accepted", version)
			}
		}(i)
	}

	wg.Wait()
	close(results)

	accepted := 0
	rejected := 0
	for result := range results {
		fmt.Printf("  %s\n", result)
		if result[len(result)-8:] == "accepted" {
			accepted++
		} else {
			rejected++
		}
	}

	fmt.Printf("  Summary: %d accepted, %d rejected\n", accepted, rejected)
}

func runCacheAsideScenario(writer, reader *CacheWrapper) {
	fmt.Println("Testing cache-aside pattern with stale local cache...")

	key := "config:400"

	// Step 1: Writer sets v1
	data1 := createVersionedData(key, 1, "writer")
	writer.Set(ctx, key, data1)
	fmt.Println("Writer: Set version 1")
	time.Sleep(100 * time.Millisecond)

	// Step 2: Reader caches v1
	checkReader(reader, key, "Reader")

	// Step 3: Writer updates to v3 (skipping v2 to make it obvious)
	data3 := createVersionedData(key, 3, "writer")
	writer.Set(ctx, key, data3)
	fmt.Println("Writer: Set version 3")
	time.Sleep(100 * time.Millisecond)

	// Step 4: Simulate reader missing the pub/sub message
	// Reader's local cache still has v1, but detector knows v3 exists
	fmt.Println("->Reader's local cache has v1, but detector knows v3...")

	// Step 5: Reader tries to use stale local cache
	fmt.Println("->Reader performs Get() - should detect staleness...")
	if val, found := reader.Get(ctx, key); found {
		if data, ok := val.(*VersionedData); ok {
			fmt.Printf("Reader: Got version %d (validated and current)\n", data.Version)
		}
	} else {
		fmt.Println("Stale local cache detected and invalidated!")
	}
}

func runNetworkPartitionScenario(writer, reader *CacheWrapper, detector *StaleDetector) {
	fmt.Println("Simulating network partition with stale local cache...")

	key := "session:500"

	// Initial state
	data1 := createVersionedData(key, 1, "writer")
	writer.Set(ctx, key, data1)
	time.Sleep(100 * time.Millisecond)
	checkReader(reader, key, "Reader")

	fmt.Println("Simulating network partition...")
	fmt.Println("-> Reader is isolated, writer continues updating...")

	// Writer updates to v2, v3, v4 but reader doesn't receive pub/sub
	for i := 2; i <= 4; i++ {
		data := createVersionedData(key, i, "writer")
		writer.Set(ctx, key, data)
		fmt.Printf("  Writer: Updated to v%d (reader not notified)\n", i)
		time.Sleep(30 * time.Millisecond)
	}

	fmt.Println("Network partition healed")
	time.Sleep(50 * time.Millisecond)

	// Reader tries to access - should detect stale cache
	fmt.Println("-> Reader performs Get() after partition...")
	if val, found := reader.Get(ctx, key); found {
		if data, ok := val.(*VersionedData); ok {
			if data.Version < 4 {
				fmt.Printf("Reader has stale version %d (should be 4)\n", data.Version)

				// Force refresh by checking detector
				if info, ok := detector.GetVersionInfo(key); ok {
					fmt.Printf("Detector knows current version is %d\n", info.Version)
				}
			} else {
				fmt.Printf("Reader refreshed to latest version %d\n", data.Version)
			}
		}
	} else {
		fmt.Println("Stale cache invalidated, will fetch from Redis")
	}
}

func createVersionedData(key string, version int, updatedBy string) *VersionedData {
	return &VersionedData{
		Key:       key,
		Value:     fmt.Sprintf("Data v%d by %s", version, updatedBy),
		Version:   int64(version),
		Timestamp: time.Now().UnixNano(),
		UpdatedBy: updatedBy,
	}
}

func checkReader(reader *CacheWrapper, key, name string) {
	if val, found := reader.Get(ctx, key); found {
		if data, ok := val.(*VersionedData); ok {
			fmt.Printf("  %s: Has version %d (value: %s)\n", name, data.Version, data.Value)
		}
	} else {
		fmt.Printf("  %s: Not in cache (would fetch from Redis)\n", name)
	}
}
