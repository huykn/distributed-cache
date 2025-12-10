// Package main demonstrates improved stale data prevention with proper test scenarios
// that actually validate version conflicts and cache-aside pattern issues.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dc "github.com/huykn/distributed-cache"
	"github.com/huykn/distributed-cache/cache"
)

// VersionedData represents data with version and timestamp.
type VersionedData struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Version   int64  `json:"version"`
	Timestamp int64  `json:"timestamp"`
	UpdatedBy string `json:"updated_by"`
}

// TestResult tracks test outcomes for verification.
type TestResult struct {
	Name        string
	Passed      bool
	Expected    string
	Actual      string
	Description string
}

// StaleDetector with explicit tracking of accepts/rejects.
type StaleDetector struct {
	latestVersions  sync.Map
	staleRejections int64 // Actually rejected stale data
	duplicates      int64 // Same version (not stale, just duplicate)
	freshAccepts    int64 // Fresh data accepted
	totalChecks     int64
	logger          cache.Logger
}

type VersionInfo struct {
	Version   int64
	Timestamp int64
	Source    string
}

func NewStaleDetector(logger cache.Logger) *StaleDetector {
	return &StaleDetector{logger: logger}
}

// CompareAndRecord with explicit categorization.
func (sd *StaleDetector) CompareAndRecord(key string, version, timestamp int64, source string) (bool, string) {
	atomic.AddInt64(&sd.totalChecks, 1)

	newInfo := &VersionInfo{Version: version, Timestamp: timestamp, Source: source}

	for {
		current, loaded := sd.latestVersions.LoadOrStore(key, newInfo)
		if !loaded {
			atomic.AddInt64(&sd.freshAccepts, 1)
			return true, "FRESH"
		}

		currentInfo := current.(*VersionInfo)

		if version > currentInfo.Version {
			if sd.latestVersions.CompareAndSwap(key, current, newInfo) {
				atomic.AddInt64(&sd.freshAccepts, 1)
				sd.logger.Info("OK ACCEPTED newer version",
					"key", key, "v", fmt.Sprintf("%d->%d", currentInfo.Version, version), "source", source)
				return true, "NEWER"
			}
			continue
		}

		if version < currentInfo.Version {
			atomic.AddInt64(&sd.staleRejections, 1)
			sd.logger.Warn("NG REJECTED stale data",
				"key", key, "stale_v", version, "current_v", currentInfo.Version, "source", source)
			return false, "STALE"
		}

		// Same version
		atomic.AddInt64(&sd.duplicates, 1)
		return false, "DUPLICATE"
	}
}

func (sd *StaleDetector) GetStats() (stale, duplicates, fresh, total int64) {
	return atomic.LoadInt64(&sd.staleRejections),
		atomic.LoadInt64(&sd.duplicates),
		atomic.LoadInt64(&sd.freshAccepts),
		atomic.LoadInt64(&sd.totalChecks)
}

func (sd *StaleDetector) GetVersion(key string) (int64, bool) {
	val, ok := sd.latestVersions.Load(key)
	if !ok {
		return 0, false
	}
	return val.(*VersionInfo).Version, true
}

// CacheWrapper with state verification.
type CacheWrapper struct {
	cache.Cache
	detector *StaleDetector
	podID    string
	logger   cache.Logger
}

func NewCacheWrapper(c cache.Cache, detector *StaleDetector, podID string, logger cache.Logger) *CacheWrapper {
	return &CacheWrapper{
		Cache:    c,
		detector: detector,
		podID:    podID,
		logger:   logger,
	}
}

// Set validates version before setting to cache.
func (cw *CacheWrapper) Set(ctx context.Context, key string, value interface{}) error {
	data, ok := value.(*VersionedData)
	if !ok {
		return errors.New("value must be *VersionedData")
	}

	shouldAccept, reason := cw.detector.CompareAndRecord(key, data.Version, data.Timestamp, cw.podID+":set")
	if !shouldAccept {
		return fmt.Errorf("rejected: %s (version %d)", reason, data.Version)
	}

	return cw.Cache.Set(ctx, key, value)
}

func (cw *CacheWrapper) GetWithVersion(ctx context.Context, key string) (*VersionedData, bool) {
	val, found := cw.Cache.Get(ctx, key)
	if !found {
		return nil, false
	}

	data, ok := val.(*VersionedData)
	if !ok {
		return nil, false
	}

	// Validate cached data against detector's known version
	shouldAccept, reason := cw.detector.CompareAndRecord(
		key, data.Version, data.Timestamp, cw.podID+":get-validate")

	if !shouldAccept && reason == "STALE" {
		// Stale data detected in local cache - invalidate it
		cw.logger.Warn("Stale cache invalidated on Get",
			"key", key, "cached_v", data.Version, "pod", cw.podID)
		cw.Cache.Delete(ctx, key)
		return nil, false
	}

	return data, true
}

var ctx = context.Background()

func main() {
	fmt.Println("Stale Data Prevention - Verification Test")

	logger := cache.NewConsoleLogger("demo")
	detector := NewStaleDetector(logger)

	results := []TestResult{}

	writer := createCache("writer", true, logger, detector)
	defer writer.Close()

	reader1 := createCache("reader-1", false, logger, detector)
	defer reader1.Close()

	reader2 := createCache("reader-2", false, logger, detector)
	defer reader2.Close()

	// Test 1: Basic version ordering
	fmt.Println("\n[TEST 1] Sequential Updates - Verify Correct Ordering")
	fmt.Println("─────────────────────────────────────────────────────────")
	results = append(results, testSequentialUpdates(writer, reader1, detector))

	time.Sleep(200 * time.Millisecond)

	// Test 2: Explicit stale data injection
	fmt.Println("\n[TEST 2] Stale Data Injection - Verify Rejection")
	fmt.Println("─────────────────────────────────────────────────────────")
	results = append(results, testStaleDataInjection(writer, reader1, detector))

	time.Sleep(200 * time.Millisecond)

	// Test 3: Out-of-order pub/sub
	fmt.Println("\n[TEST 3] Out-of-Order Delivery - Verify Correct Final State")
	fmt.Println("─────────────────────────────────────────────────────────")
	results = append(results, testOutOfOrderDelivery(writer, reader1, detector))

	time.Sleep(200 * time.Millisecond)

	// Test 4: Cache-aside staleness
	fmt.Println("\n[TEST 4] Cache-Aside Pattern - Verify Stale Detection")
	fmt.Println("─────────────────────────────────────────────────────────")
	results = append(results, testCacheAsideStaleness(writer, reader1, detector))

	time.Sleep(200 * time.Millisecond)

	// Test 5: Concurrent races
	fmt.Println("\n[TEST 5] Concurrent Races - Verify Monotonic Versions")
	fmt.Println("─────────────────────────────────────────────────────────")
	results = append(results, testConcurrentRaces(writer, detector))

	// Test 6: Active stale detection on Get()
	fmt.Println("\n[TEST 6] Active Stale Detection on Get() - Verify Cache Invalidation")
	fmt.Println("─────────────────────────────────────────────────────────")
	results = append(results, testActiveStaleDetectionOnGet(writer, reader1, detector))

	// Print summary
	printTestSummary(results, detector)
}

func createCache(podID string, canWrite bool, logger cache.Logger, detector *StaleDetector) *CacheWrapper {
	cfg := dc.DefaultConfig()
	cfg.PodID = podID
	cfg.RedisAddr = "localhost:6379"
	cfg.InvalidationChannel = "verification-test"
	cfg.DebugMode = false
	cfg.Logger = cache.NewConsoleLogger("quiet") // Reduce noise
	cfg.ReaderCanSetToRedis = canWrite

	cfg.OnSetLocalCache = func(event dc.InvalidationEvent) any {
		var data VersionedData
		if err := json.Unmarshal(event.Value, &data); err != nil {
			return nil
		}

		shouldAccept, reason := detector.CompareAndRecord(
			data.Key, data.Version, data.Timestamp, podID+":pubsub")

		if !shouldAccept && reason == "STALE" {
			logger.Warn("Pubsub STALE rejected", "key", data.Key, "v", data.Version)
			return nil
		}

		return &data
	}

	c, err := dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	return NewCacheWrapper(c, detector, podID, logger)
}

func testSequentialUpdates(writer, reader *CacheWrapper, detector *StaleDetector) TestResult {
	key := "test:sequential"

	fmt.Println("Step 1: Writer sets v1, v2, v3")
	for v := 1; v <= 3; v++ {
		data := &VersionedData{
			Key: key, Value: fmt.Sprintf("data-v%d", v),
			Version: int64(v), Timestamp: time.Now().UnixNano(),
			UpdatedBy: "writer",
		}
		writer.Set(ctx, key, data)
		fmt.Printf("  Writer -> v%d\n", v)
		time.Sleep(30 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nStep 2: Verify reader has v3")
	data, found := reader.GetWithVersion(ctx, key)

	expected := "v3"
	actual := "not found"
	if found {
		actual = fmt.Sprintf("v%d", data.Version)
	}

	passed := found && data.Version == 3

	fmt.Printf("  Reader cache: %s\n", actual)
	fmt.Printf("  Detector version: v%d\n", mustGetVersion(detector, key))

	return TestResult{
		Name:        "Sequential Updates",
		Passed:      passed,
		Expected:    expected,
		Actual:      actual,
		Description: "Reader should have latest version v3",
	}
}

func testStaleDataInjection(writer, reader *CacheWrapper, detector *StaleDetector) TestResult {
	key := "test:injection"

	fmt.Println("Step 1: Writer sets v5")
	data5 := &VersionedData{
		Key: key, Value: "current-v5",
		Version: 5, Timestamp: time.Now().UnixNano(),
		UpdatedBy: "writer",
	}
	writer.Set(ctx, key, data5)
	fmt.Println("  Writer -> v5")

	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nStep 2: Try to inject stale v2")
	staleData := &VersionedData{
		Key: key, Value: "STALE-v2",
		Version: 2, Timestamp: time.Now().UnixNano(),
		UpdatedBy: "attacker",
	}

	err := reader.Set(ctx, key, staleData)
	rejected := err != nil && strings.Contains(err.Error(), "STALE")

	if rejected {
		fmt.Println("  OK Stale v2 REJECTED")
	} else {
		fmt.Println("  NG Stale v2 ACCEPTED (BUG!)")
	}

	fmt.Println("\nStep 3: Verify reader still has v5")
	data, found := reader.GetWithVersion(ctx, key)

	expected := "v5"
	actual := "not found"
	if found {
		actual = fmt.Sprintf("v%d", data.Version)
	}

	passed := rejected && found && data.Version == 5

	fmt.Printf("  Reader cache: %s (%s)\n", actual, data.Value)
	fmt.Printf("  Detector version: v%d\n", mustGetVersion(detector, key))

	return TestResult{
		Name:        "Stale Data Injection",
		Passed:      passed,
		Expected:    expected,
		Actual:      actual,
		Description: "Stale v2 should be rejected, cache should remain v5",
	}
}

func testOutOfOrderDelivery(writer, reader *CacheWrapper, detector *StaleDetector) TestResult {
	key := "test:outoforder"

	fmt.Println("Step 1: Writer sends v1, v2, v3, v4, v5 rapidly")
	for v := 1; v <= 5; v++ {
		data := &VersionedData{
			Key: key, Value: fmt.Sprintf("msg-v%d", v),
			Version: int64(v), Timestamp: time.Now().UnixNano(),
			UpdatedBy: "writer",
		}
		writer.Set(ctx, key, data)
		time.Sleep(5 * time.Millisecond) // Very fast to cause reordering
	}

	time.Sleep(150 * time.Millisecond)

	fmt.Println("\nStep 2: Simulate delayed v2 arriving late")
	delayedV2 := &VersionedData{
		Key: key, Value: "DELAYED-v2",
		Version: 2, Timestamp: time.Now().Add(-1 * time.Second).UnixNano(),
		UpdatedBy: "delayed",
	}

	shouldAccept, reason := detector.CompareAndRecord(key, 2, delayedV2.Timestamp, "test:delayed")
	rejected := !shouldAccept && reason == "STALE"

	if rejected {
		fmt.Println("  OK Delayed v2 REJECTED")
	} else {
		fmt.Printf("  NG Delayed v2 %s (expected REJECTED)\n", reason)
	}

	fmt.Println("\nStep 3: Verify final state is v5")
	data, found := reader.GetWithVersion(ctx, key)

	expected := "v5"
	actual := "not found"
	if found {
		actual = fmt.Sprintf("v%d", data.Version)
	}

	passed := rejected && found && data.Version == 5

	fmt.Printf("  Reader cache: %s\n", actual)
	fmt.Printf("  Detector version: v%d\n", mustGetVersion(detector, key))

	return TestResult{
		Name:        "Out-of-Order Delivery",
		Passed:      passed,
		Expected:    expected,
		Actual:      actual,
		Description: "Delayed v2 should be rejected, final state should be v5",
	}
}

func testCacheAsideStaleness(writer, reader *CacheWrapper, detector *StaleDetector) TestResult {
	key := "test:cacheaside"

	fmt.Println("Step 1: Initial state - writer sets v1")
	data1 := &VersionedData{
		Key: key, Value: "initial-v1",
		Version: 1, Timestamp: time.Now().UnixNano(),
		UpdatedBy: "writer",
	}
	writer.Set(ctx, key, data1)
	time.Sleep(150 * time.Millisecond) // Wait for pub/sub propagation

	// Reader caches v1
	cached, found := reader.GetWithVersion(ctx, key)
	if found {
		fmt.Printf("  Reader cached v%d\n", cached.Version)
	}

	fmt.Println("\nStep 2: Writer updates to v10")
	data10 := &VersionedData{
		Key: key, Value: "updated-v10",
		Version: 10, Timestamp: time.Now().UnixNano(),
		UpdatedBy: "writer",
	}
	writer.Set(ctx, key, data10)
	fmt.Println("  Writer -> v10 (pub/sub will propagate)")

	time.Sleep(150 * time.Millisecond) // Wait for pub/sub propagation

	fmt.Println("\nStep 3: Reader performs Get() after pub/sub")

	// Reader should receive v10 via pub/sub, OR detect staleness on Get
	cachedData, foundInCache := reader.GetWithVersion(ctx, key)
	detectorVersion := mustGetVersion(detector, key)

	var actualVersion int64
	status := "not found"

	if foundInCache {
		actualVersion = cachedData.Version
		status = fmt.Sprintf("v%d", actualVersion)

		if actualVersion == detectorVersion {
			fmt.Printf("  OK Reader has current v%d (via pub/sub or cache refresh)\n", actualVersion)
		} else if actualVersion < detectorVersion {
			fmt.Printf("  WN Reader has stale v%d (detector knows v%d)\n", actualVersion, detectorVersion)
		}
	} else {
		fmt.Println("  OK Cache invalidated (will fetch fresh from Redis)")
	}

	fmt.Printf("  Detector version: v%d\n", detectorVersion)

	expected := fmt.Sprintf("v%d", detectorVersion)
	actual := status

	// Pass if reader has latest version OR cache was invalidated
	passed := !foundInCache || actualVersion == detectorVersion

	description := "Reader should have latest version via pub/sub or detect staleness"
	if !passed {
		description = fmt.Sprintf("FAIL: Reader has v%d but detector knows v%d", actualVersion, detectorVersion)
	}

	return TestResult{
		Name:        "Cache-Aside Pattern",
		Passed:      passed,
		Expected:    expected,
		Actual:      actual,
		Description: description,
	}
}

func testActiveStaleDetectionOnGet(writer, reader *CacheWrapper, detector *StaleDetector) TestResult {
	key := "test:active-detection"

	fmt.Println("Step 1: Reader manually caches stale data")
	// Simulate reader having old cached data (bypass pub/sub)
	staleData := &VersionedData{
		Key: key, Value: "stale-v3",
		Version: 3, Timestamp: time.Now().UnixNano(),
		UpdatedBy: "old-writer",
	}

	// Directly set to reader's local cache (simulating missed pub/sub)
	reader.Cache.Set(ctx, key, staleData)
	fmt.Println("  Reader has v3 in local cache (simulating missed update)")

	fmt.Println("\nStep 2: Update detector to know v8 exists")
	// Simulate that v8 is the current version in Redis
	detector.CompareAndRecord(key, 8, time.Now().UnixNano(), "redis:truth")
	fmt.Println("  Detector now knows v8 is current")

	fmt.Println("\nStep 3: Reader performs Get() - should detect staleness")

	// This Get() should trigger validation and detect stale cache
	cachedData, foundInCache := reader.GetWithVersion(ctx, key)
	detectorVersion := mustGetVersion(detector, key)

	var actualVersion int64
	status := "invalidated"
	staleDetected := false

	if foundInCache {
		actualVersion = cachedData.Version
		status = fmt.Sprintf("v%d", actualVersion)

		if actualVersion < detectorVersion {
			fmt.Printf("  NG BUG: Reader still has stale v%d (should be invalidated)\n", actualVersion)
		} else {
			fmt.Printf("  OK Reader has v%d\n", actualVersion)
		}
	} else {
		staleDetected = true
		fmt.Println("  OK Stale cache detected and invalidated!")
		fmt.Println("  -> Reader would now fetch fresh v8 from Redis")
	}

	fmt.Printf("  Detector version: v%d\n", detectorVersion)

	expected := "invalidated (stale v3 removed)"
	actual := status

	// Pass if cache was invalidated (foundInCache = false)
	passed := staleDetected

	description := "Get() should actively detect and invalidate stale local cache"
	if !passed {
		description = fmt.Sprintf("FAIL: Get() returned stale v%d instead of invalidating", actualVersion)
	}

	return TestResult{
		Name:        "Active Stale Detection",
		Passed:      passed,
		Expected:    expected,
		Actual:      actual,
		Description: description,
	}
}

func testConcurrentRaces(writer *CacheWrapper, detector *StaleDetector) TestResult {
	key := "test:concurrent"

	fmt.Println("Step 1: Launch 10 concurrent writers")

	var wg sync.WaitGroup
	versions := []int{7, 3, 9, 1, 5, 10, 2, 8, 4, 6}

	for _, v := range versions {
		wg.Add(1)
		go func(version int) {
			defer wg.Done()
			data := &VersionedData{
				Key: key, Value: fmt.Sprintf("race-v%d", version),
				Version: int64(version), Timestamp: time.Now().UnixNano(),
				UpdatedBy: fmt.Sprintf("racer-%d", version),
			}
			err := writer.Set(ctx, key, data)
			if err != nil {
				fmt.Printf("  v%d rejected\n", version)
			}
		}(v)
		time.Sleep(2 * time.Millisecond)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nStep 2: Verify final version is maximum (v10)")
	finalVersion := mustGetVersion(detector, key)

	expected := "v10"
	actual := fmt.Sprintf("v%d", finalVersion)
	passed := finalVersion == 10

	fmt.Printf("  Final version: v%d\n", finalVersion)
	fmt.Printf("  All lower versions should have been rejected\n")

	return TestResult{
		Name:        "Concurrent Races",
		Passed:      passed,
		Expected:    expected,
		Actual:      actual,
		Description: "Under concurrent updates, only highest version should win",
	}
}

func printTestSummary(results []TestResult, detector *StaleDetector) {
	fmt.Println("TEST SUMMARY")

	passed := 0
	for i, result := range results {
		status := "NG FAIL"
		if result.Passed {
			status = "OK PASS"
			passed++
		}

		fmt.Printf("\n[%d] %s %s\n", i+1, status, result.Name)
		fmt.Printf("    Expected: %s\n", result.Expected)
		fmt.Printf("    Actual:   %s\n", result.Actual)
		fmt.Printf("    Info:     %s\n", result.Description)
	}

	stale, duplicates, fresh, total := detector.GetStats()

	fmt.Println("\nDETECTOR STATISTICS")
	fmt.Printf("  Total Checks:      %d\n", total)
	fmt.Printf("  Fresh Accepts:     %d (new/newer versions)\n", fresh)
	fmt.Printf("  Stale Rejections:  %d (<-- THIS IS THE KEY METRIC)\n", stale)
	fmt.Printf("  Duplicates:        %d (same version, not stale)\n", duplicates)

	fmt.Printf("\nFINAL RESULT: %d/%d tests passed ", passed, len(results))
	if passed == len(results) {
		fmt.Println("\nOK Stale data prevention is VERIFIED")
	} else {
		fmt.Println("\nNG Some tests FAILED - review above")
	}
}

func mustGetVersion(detector *StaleDetector, key string) int64 {
	v, ok := detector.GetVersion(key)
	if !ok {
		return 0
	}
	return v
}
