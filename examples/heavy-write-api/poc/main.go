package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"net/http"
	_ "net/http/pprof"

	_ "github.com/go-sql-driver/mysql"
)

// VoucherDBConfig holds MySQL voucher table configuration
type VoucherDBConfig struct {
	TotalVouchers int64
	ValidFrom     time.Time
	ValidUntil    time.Time
}

// Default configuration
const (
	DefaultNumPods        = 2
	DefaultMySQLDSN       = "root:123456@tcp(localhost:3307)/voucher_db"
	DefaultTotalSimulated = 30_000_000 // 30M vouchers = 50k req/s * 60s * 2 pods * 5 minutes
)

func main() {
	// Parse command line flags
	demoSeconds := flag.Int("demo", 0, "Run in demo mode for N seconds (0 = run until Ctrl+C)")
	chaosEngineering := flag.Int("chaos", 0, "Run in chaos engineering mode")
	redisAddr := flag.String("redis", DefaultRedisAddr, "Redis server address")
	flag.Parse()

	log.Println("Starting Heavy Write API POC Coordinator")

	go func() {
		http.ListenAndServe("localhost:6060", nil)
		// go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap
	}()

	// Initialize Redis-backed GlobalBus
	if err := InitGlobalBus(*redisAddr); err != nil {
		log.Fatalf("[Coordinator] Failed to connect to Redis: %v", err)
	}
	defer CloseGlobalBus()

	log.Println("[Coordinator] Clearing Redis (FLUSHDB)...")
	globalBus := GetGlobalBus()
	globalBus.Clear()
	log.Println("[Coordinator] Redis cleared")

	demoTimeout := *demoSeconds

	// 1. Setup Context with Graceful Shutdown
	var ctx context.Context
	var cancel context.CancelFunc
	if demoTimeout > 0 {
		log.Printf("[Coordinator] Demo mode: will run for %d seconds", demoTimeout)
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(demoTimeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 2. Connect to MySQL (optional - will fallback to simulation)
	db, dbConfig := connectToMySQL()
	if db != nil {
		defer db.Close()
	}

	log.Printf("[Coordinator] Total vouchers: %d", dbConfig.TotalVouchers)

	// 3. Calculate pod configuration
	numPods := DefaultNumPods
	vouchersPerPod := dbConfig.TotalVouchers / int64(numPods)
	log.Printf("[Coordinator] Pods: %d, Vouchers per pod: %d", numPods, vouchersPerPod)
	log.Printf("[Coordinator] Each pod can handle ~50k req/s, %d pods = %dk req/s capacity", numPods, numPods*50)

	// 4. Start Worker Service
	worker := NewWorker(dbConfig.TotalVouchers, numPods, db)
	var wg sync.WaitGroup
	wg.Go(func() {
		worker.Run(ctx)
	})

	// 5. Start Read Pods with calculated ranges
	pods := make([]*ReadPod, numPods)
	for i := range numPods {
		startID := int64(i)*vouchersPerPod + 1
		endID := int64(i+1) * vouchersPerPod
		pods[i] = NewReadPod(i, numPods, dbConfig.TotalVouchers, startID, endID)
		log.Printf("[Coordinator] Pod %d serving voucher range [%d, %d]", i, startID, endID)
		wg.Add(1)
		go func(p *ReadPod) {
			defer wg.Done()
			p.Run(ctx)
		}(pods[i])
	}

	// 6. Chaos Engineering - Simulate Pod Kill/Restart
	podsMu := &sync.RWMutex{}
	if *chaosEngineering > 0 {
		log.Printf("[Coordinator] Chaos engineering mode enabled")
		go runChaosEngineering(ctx, &wg, pods, podsMu, dbConfig, vouchersPerPod)
	}

	// 7. Simulate High-Volume Client Traffic
	go simulateClientTraffic(ctx, pods, podsMu)

	// 8. Start Distribution Monitor
	go runDistributionMonitor(ctx, cancel, numPods, dbConfig.TotalVouchers)

	// 9. Handle graceful shutdown
	go func() {
		<-sigChan
		log.Println("Shutdown signal received. Initiating graceful shutdown...")

		for _, pod := range pods {
			pod.SaveStateToRedis()
		}
		cancel()
	}()

	wg.Wait()
	log.Println("[Coordinator] Shutdown complete.")
}

func connectToMySQL() (*sql.DB, *VoucherDBConfig) {
	log.Println("[DB] Attempting MySQL connection...")

	db, err := sql.Open("mysql", DefaultMySQLDSN)
	if err != nil {
		log.Panicf("[DB] MySQL connection failed: %v (using simulation)", err)
		return nil, &VoucherDBConfig{
			TotalVouchers: DefaultTotalSimulated,
			ValidFrom:     time.Now(),
			ValidUntil:    time.Now().Add(24 * time.Hour),
		}
	}

	// Test connection
	if err := db.Ping(); err != nil {
		log.Printf("[DB] MySQL ping failed: %v (using simulation)", err)
		db.Close()
		return nil, &VoucherDBConfig{
			TotalVouchers: DefaultTotalSimulated,
			ValidFrom:     time.Now(),
			ValidUntil:    time.Now().Add(24 * time.Hour),
		}
	}

	// Query total vouchers
	var totalVouchers int64
	err = db.QueryRow("SELECT COUNT(*) FROM voucher").Scan(&totalVouchers)
	if err != nil {
		log.Printf("[DB] Query failed: %v (using simulation)", err)
		db.Close()
		return nil, &VoucherDBConfig{
			TotalVouchers: DefaultTotalSimulated,
			ValidFrom:     time.Now(),
			ValidUntil:    time.Now().Add(24 * time.Hour),
		}
	}

	log.Printf("[DB] Connected to MySQL. Total vouchers: %d", totalVouchers)
	return db, &VoucherDBConfig{
		TotalVouchers: totalVouchers,
		ValidFrom:     time.Now(),
		ValidUntil:    time.Now().Add(24 * time.Hour),
	}
}

// runChaosEngineering simulates pod kill and restart (new pod takes over)
func runChaosEngineering(ctx context.Context, wg *sync.WaitGroup, pods []*ReadPod, podsMu *sync.RWMutex, dbConfig *VoucherDBConfig, vouchersPerPod int64) {
	time.Sleep(10 * time.Second) // Wait for system warmup
	log.Println("[Chaos] Starting chaos engineering simulation (kill/restart)...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Second): // Kill every 60 seconds
			targetIdx := rand.Intn(len(pods))
			log.Printf("[Chaos] Killing pod-%d...", targetIdx)

			podsMu.RLock()
			oldPod := pods[targetIdx]
			podsMu.RUnlock()

			// Stop the old pod (kill goroutine)
			oldPod.Kill()

			// Wait a bit to simulate downtime
			time.Sleep(2 * time.Second)

			// Create new pod (simulates new deployment taking over)
			startID := int64(targetIdx)*vouchersPerPod + 1
			endID := int64(targetIdx+1) * vouchersPerPod
			newPod := NewReadPod(targetIdx, DefaultNumPods, dbConfig.TotalVouchers, startID, endID)

			// Restore state from Redis BEFORE acquiring lock to avoid blocking traffic
			newPod.RestoreStateFromRedis()

			podsMu.Lock()
			pods[targetIdx] = newPod
			podsMu.Unlock()

			log.Printf("[Chaos] New pod-%d deployed and restored from Redis", targetIdx)

			// Start the new pod
			wg.Add(1)
			go func(p *ReadPod) {
				defer wg.Done()
				p.Run(ctx)
			}(newPod)
		}
	}
}

// simulateClientTraffic simulates high-volume client requests using concurrent goroutines to consume all distributed vouchers (10M spins target)
func simulateClientTraffic(ctx context.Context, pods []*ReadPod, podsMu *sync.RWMutex) {
	time.Sleep(3 * time.Second) // Wait for system warmup
	currentMinute := time.Now().Minute()
	log.Printf("[Traffic] Waiting for next minute from %d to %d to start traffic simulation...\n", currentMinute, currentMinute+1)
	time.Sleep(time.Duration(60-time.Now().Second()) * time.Second) // wait for next minute from now
	currentMinute = time.Now().Minute()
	log.Printf("[Traffic] Starting traffic simulation at %d...\n", currentMinute) // start traffic simulation

	const numWorkers = 20 // Concurrent goroutines hammering spins
	// set const numWorkers = 2 -> [Traffic] Spins: 10.0M, misses: 1.2M (avg: 71421 spins/s)

	var totalSpins int64
	var totalMisses int64
	startTime := time.Now()

	// Stats reporter
	go func() {
		statsTicker := time.NewTicker(5 * time.Second)
		defer statsTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsTicker.C:
				spins := atomic.LoadInt64(&totalSpins)
				misses := atomic.LoadInt64(&totalMisses)
				elapsed := time.Since(startTime).Seconds()
				if elapsed > 0 {
					log.Printf("[Traffic] Spins: %s, misses: %s (avg: %.0f spins/s)",
						formatMillions(int64(spins)), formatMillions(int64(misses)),
						float64(spins)/elapsed)
				}
			}
		}
	}()

	// Launch concurrent spin workers
	var wg sync.WaitGroup
	for w := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localSpins := int64(0)
			localMisses := int64(0)

			// Rate limiter per worker to respect PodServeRate (50K/s) across all workers
			// targetRPS := PodServeRate * len(pods) / numWorkers
			// spinInterval := time.Second / time.Duration(targetRPS)
			// ticker := time.NewTicker(spinInterval)
			ticker := time.NewTicker(1)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					atomic.AddInt64(&totalSpins, localSpins)
					atomic.AddInt64(&totalMisses, localMisses)
					return
				case <-ticker.C:
				}

				podsMu.RLock()
				podIdx := rand.Intn(len(pods))
				pod := pods[podIdx]
				podsMu.RUnlock()

				userID := fmt.Sprintf("user-%d", rand.Intn(100000))
				_, _, ok := pod.HandleSpin(userID)
				if ok {
					localSpins++
					// time.Sleep(1 * time.Microsecond)
				} else {
					localMisses++
					// Brief sleep on miss to avoid busy-spinning when no vouchers available
					time.Sleep(100 * time.Microsecond)
				}

				// Batch update global counters every 1000 spins
				if localSpins%1000 == 0 && localSpins > 0 {
					atomic.AddInt64(&totalSpins, localSpins)
					localSpins = 0
				}
				if localMisses%1000 == 0 && localMisses > 0 {
					atomic.AddInt64(&totalMisses, localMisses)
					localMisses = 0
				}
			}
		}(w)
	}

	wg.Wait()
	spins := atomic.LoadInt64(&totalSpins)
	elapsed := time.Since(startTime).Seconds()
	log.Printf("[Traffic] Total spins: %s (avg: %.0f spins/s)", formatMillions(spins), float64(spins)/elapsed)
}

// runDistributionMonitor tracks voucher distribution progress across all pods and triggers graceful shutdown when all vouchers are distributed and claimed.
func runDistributionMonitor(ctx context.Context, cancel context.CancelFunc, numPods int, totalVouchers int64) {
	time.Sleep(5 * time.Second) // Wait for system warmup
	log.Println("[Monitor] Starting distribution monitor...")

	const (
		pollInterval       = 10 * time.Second
		claimWaitTimeout   = 60 * time.Second // Max wait for final claims after all distributed
		voucherIDTrackSize = 100              // Sample size for range verification
	)

	globalBus := GetGlobalBus()
	allDistributed := false
	allDistributedAt := time.Time{}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1. Aggregate pod health data (spins + claims)
			var totalSpins, totalClaims int64
			podStatuses := make([]podMonitorStatus, 0, numPods)

			for i := range numPods {
				slotKey := fmt.Sprintf("pod:health:%d", i)
				data, exists := globalBus.Get(slotKey)
				if !exists {
					continue
				}

				var status map[string]any
				if err := json.Unmarshal(data, &status); err != nil {
					continue
				}

				spins := int64(0)
				claims := int64(0)
				if v, ok := status["spins"].(float64); ok {
					spins = int64(v)
				}
				if v, ok := status["claims"].(float64); ok {
					claims = int64(v)
				}
				startID := int64(0)
				endID := int64(0)
				if v, ok := status["start_id"].(float64); ok {
					startID = int64(v)
				}
				if v, ok := status["end_id"].(float64); ok {
					endID = int64(v)
				}

				totalSpins += spins
				totalClaims += claims
				podStatuses = append(podStatuses, podMonitorStatus{
					podIndex: i,
					spins:    spins,
					claims:   claims,
					startID:  startID,
					endID:    endID,
				})
			}

			// 2. Aggregate distribution range progress from worker
			var totalDistributed int64
			allRangesExhausted := true
			rangeDetails := make([]rangeMonitorStatus, 0, numPods)

			for i := range numPods {
				podID := fmt.Sprintf("pod-%d", i)
				rangeKey := fmt.Sprintf("dist-range:%s", podID)
				data, exists := globalBus.Get(rangeKey)
				if !exists {
					allRangesExhausted = false
					continue
				}

				var rangeData map[string]float64
				if err := json.Unmarshal(data, &rangeData); err != nil {
					allRangesExhausted = false
					continue
				}

				currentID := int64(rangeData["current_id"])
				endID := int64(rangeData["end_id"])
				startID := int64(rangeData["start_id"])
				distributed := int64(rangeData["distributed"])

				totalDistributed += distributed

				if currentID <= endID {
					allRangesExhausted = false
				}

				rangeDetails = append(rangeDetails, rangeMonitorStatus{
					podIndex:    i,
					startID:     startID,
					endID:       endID,
					currentID:   currentID,
					distributed: distributed,
					exhausted:   currentID > endID,
				})
			}

			// 3. Verify voucher ID range correctness (sample check)
			verifyVoucherRangeDelivery(globalBus, rangeDetails, podStatuses)

			// 4. Log progress
			distPct := float64(totalDistributed) / float64(totalVouchers) * 100
			claimRate := float64(0)
			if totalSpins > 0 {
				claimRate = float64(totalClaims) / float64(totalSpins) * 100
			}

			log.Printf("[Monitor] Progress: %s/%s vouchers distributed to pods (%.1f%%), %s spins, %s claimed (%.1f%% claim rate)",
				formatMillions(totalDistributed), formatMillions(totalVouchers), distPct,
				formatMillions(totalSpins), formatMillions(totalClaims), claimRate)

			// Log per-pod range status
			for _, r := range rangeDetails {
				status := "(pending)"
				if r.exhausted {
					status = "(ready)"
				}
				log.Printf("[Monitor] %s Pod-%d: range [%d-%d], current=%d, distributed=%d",
					status, r.podIndex, r.startID, r.endID, r.currentID, r.distributed)
			}

			// Log per-pod minute store status (current serving + future buffer building)
			for _, ps := range podStatuses {
				slotKey := fmt.Sprintf("pod:health:%d", ps.podIndex)
				data, exists := globalBus.Get(slotKey)
				if !exists {
					continue
				}
				var status map[string]any
				if err := json.Unmarshal(data, &status); err != nil {
					continue
				}

				currentMinTotal := int64(0)
				currentMinIdx := int64(0)
				nextMinTotal := int64(0)
				nextMinIdx := int64(0)
				if v, ok := status["current_min_total"].(float64); ok {
					currentMinTotal = int64(v)
				}
				if v, ok := status["current_min_idx"].(float64); ok {
					currentMinIdx = int64(v)
				}
				if v, ok := status["next_min_total"].(float64); ok {
					nextMinTotal = int64(v)
				}
				if v, ok := status["next_min_idx"].(float64); ok {
					nextMinIdx = int64(v)
				}

				// Current minute: how many vouchers have been served vs available
				available := currentMinTotal
				served := currentMinIdx
				if available > 0 {
					pct := float64(served) / float64(available) * 100
					log.Printf("[Monitor] Pod-%d current minute: served %s/%s vouchers (%.1f%%)",
						ps.podIndex, formatMillions(served), formatMillions(available), pct)
				} else {
					log.Printf("[Monitor] Pod-%d current minute: no vouchers loaded yet", ps.podIndex)
				}

				// Future minute (minute+1): buffer building status
				unconsumed := max(nextMinTotal-nextMinIdx, 0)
				bufferCap := int64(PodServeRate * 60)
				bufferForNextMinute := min(unconsumed, bufferCap)
				pct := float64(bufferForNextMinute) / float64(bufferCap) * 100
				log.Printf("[Monitor] Pod-%d future minute (minute+1): buffer %s/%s vouchers loaded (%.1f%%, total unconsumed: %s)",
					ps.podIndex, formatMillions(bufferForNextMinute), formatMillions(bufferCap), pct, formatMillions(unconsumed))
			}

			// 5. Check completion: all ranges exhausted (all vouchers pushed to pods)
			if allRangesExhausted && len(rangeDetails) == numPods && !allDistributed {
				allDistributed = true
				log.Printf("[Monitor] All %s vouchers distributed to pods! Waiting for all spins to complete...",
					formatMillions(totalVouchers))
			}

			// 6. Check final exit: all distributed AND all spins consumed
			if allDistributed {
				spinPct := float64(totalSpins) / float64(totalVouchers) * 100

				if totalSpins >= totalVouchers {
					// All vouchers have been spun — wait briefly for final claims to flush
					if !allDistributedAt.IsZero() {
						waitElapsed := time.Since(allDistributedAt)
						if waitElapsed >= claimWaitTimeout {
							claimRate := float64(0)
							if totalSpins > 0 {
								claimRate = float64(totalClaims) / float64(totalSpins) * 100
							}
							log.Printf("[Monitor] All done: %s distributed, %s spins, %s claimed (%.1f%% claim rate). Shutting down...",
								formatMillions(totalDistributed), formatMillions(totalSpins), formatMillions(totalClaims), claimRate)
							cancel()
							return
						}
						remaining := totalSpins - totalClaims
						log.Printf("[Monitor] All %s spins done. Waiting for claims: %s claimed, %s remaining (timeout in %v)...",
							formatMillions(totalSpins), formatMillions(totalClaims), formatMillions(remaining), claimWaitTimeout-waitElapsed)
					} else {
						allDistributedAt = time.Now()
						log.Printf("[Monitor] All %s spins completed! Waiting %v for final claims to flush...",
							formatMillions(totalSpins), claimWaitTimeout)
					}
				} else {
					log.Printf("[Monitor] Spins progress: %s/%s (%.1f%%), %s claimed",
						formatMillions(totalSpins), formatMillions(totalVouchers), spinPct, formatMillions(totalClaims))
				}
			}
		}
	}
}

// podMonitorStatus holds aggregated pod health data for monitoring
type podMonitorStatus struct {
	podIndex int
	spins    int64
	claims   int64
	startID  int64
	endID    int64
}

// rangeMonitorStatus holds distribution range progress for monitoring
type rangeMonitorStatus struct {
	podIndex    int
	startID     int64
	endID       int64
	currentID   int64
	distributed int64
	exhausted   bool
}

// verifyVoucherRangeDelivery checks that voucher ID ranges are being delivered correctly.
// It verifies that distributed ranges don't overlap and that fragment ranges from minute migrations are tracked properly.
func verifyVoucherRangeDelivery(globalBus *GlobalBus, ranges []rangeMonitorStatus, pods []podMonitorStatus) {
	if len(ranges) == 0 {
		return
	}

	// Check 1: Verify no range overlaps between pods
	for i := range ranges {
		for j := i + 1; j < len(ranges); j++ {
			ri := ranges[i]
			rj := ranges[j]
			// Ranges overlap if one starts before the other ends
			if ri.startID <= rj.endID && rj.startID <= ri.endID {
				log.Printf("[Monitor] WARNING: RANGE OVERLAP detected: Pod-%d [%d-%d] overlaps Pod-%d [%d-%d]",
					ri.podIndex, ri.startID, ri.endID, rj.podIndex, rj.startID, rj.endID)
			}
		}
	}

	// Check 2: Verify currentID is within valid bounds for each pod
	for _, r := range ranges {
		if r.currentID < r.startID && r.currentID != r.startID {
			log.Printf("[Monitor] WARNING: INVALID RANGE: Pod-%d currentID=%d is before startID=%d",
				r.podIndex, r.currentID, r.startID)
		}
		// currentID can be endID+1 when exhausted, that's valid
		if r.currentID > r.endID+1 {
			log.Printf("[Monitor] WARNING: INVALID RANGE: Pod-%d currentID=%d exceeds endID=%d by more than 1",
				r.podIndex, r.currentID, r.endID)
		}
	}

	// Check 3: Verify fragment ranges from minute migrations are tracked
	for _, r := range ranges {
		podID := fmt.Sprintf("pod-%d", r.podIndex)
		fragKey := fmt.Sprintf("pod-fragments:%s", podID)
		if fragData, exists := globalBus.Get(fragKey); exists {
			var fragments []FragmentRange
			if err := json.Unmarshal(fragData, &fragments); err == nil && len(fragments) > 0 {
				for _, frag := range fragments {
					if frag.Start < r.startID || frag.End > r.endID {
						log.Printf("[Monitor] WARNING: FRAGMENT OUT OF RANGE: Pod-%d fragment [%d-%d] outside assigned range [%d-%d]",
							r.podIndex, frag.Start, frag.End, r.startID, r.endID)
					}
				}
			}
		}
	}

	// Check 4: Verify spins don't exceed distributed vouchers per pod
	podDistMap := make(map[int]int64)
	for _, r := range ranges {
		podDistMap[r.podIndex] = r.distributed
	}
	for _, p := range pods {
		if dist, ok := podDistMap[p.podIndex]; ok {
			if p.spins > dist && dist > 0 {
				log.Printf("[Monitor] Pod-%d spins (%d) exceed distributed (%d) - possible counter drift from restarts",
					p.podIndex, p.spins, dist)
			}
		}
	}
}

// formatMillions formats a number as a readable string with M/K suffix
func formatMillions(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
