package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/huykn/distributed-cache/types"
)

// Channel constants for pub/sub communication
const (
	VoucherDistributionChannel = "voucher-distribution"
	PodRegistrationChannel     = "pod-registration"
	RedemptionChannel          = "voucher-redemption"
	PodHealthKeyPrefix         = "pod-health:"
	PodServeRate               = 50_000 // 50k requests/second per pod
	VoucherPackSize            = 50_000 // Vouchers per Redis pack key
)

// VoucherBatch represents a batch of vouchers sent from worker to a pod
type VoucherBatch struct {
	BatchID       string   `json:"batch_id"`
	AssignedPodID string   `json:"assigned_pod_id"`
	MinuteKey     int      `json:"minute_key"`
	Vouchers      [][]byte `json:"vouchers"`
	StartIdx      int64    `json:"start_idx"`
}

var poolVoucherBatch = sync.Pool{
	New: func() any {
		return &VoucherBatch{}
	},
}

// PodInfo tracks pod health and assigned voucher range
type PodInfo struct {
	PodID       string
	PodIndex    int
	LastSeen    time.Time
	StartID     int64
	EndID       int64
	IsHealthy   bool
	ServedCount int64
}

// VoucherRange tracks distribution progress for a pod
type VoucherRange struct {
	StartID           int64
	EndID             int64
	CurrentID         int64
	Distributed       int64
	LastFragmentCount int
}

// DBVoucher represents a voucher record from MySQL
type DBVoucher struct {
	ID      int64
	Code    string
	IsValid bool
}

// RedisVoucherIndex stores pre-indexed voucher bytes in Redis
type RedisVoucherIndex struct {
	VoucherID int64  `json:"voucher_id"`
	Bytes     []byte `json:"bytes"` // Pre-serialized [code][null_separator][isValid]
}

// Worker distributes vouchers to read pods and monitors health
type Worker struct {
	TotalVouchers     int64
	TotalPods         int
	VoucherDistChan   string
	Cache             *SimulatedCache
	ActivePods        map[string]*PodInfo
	mu                sync.RWMutex
	DistributedRanges map[string]*VoucherRange
	CurrentVoucherID  uint32
	DB                *sql.DB // MySQL connection
	RedisIndexed      bool    // Whether vouchers are pre-indexed in Redis
	LastDistMinute    int     // Track last distributed minute
}

// NewWorker creates a new worker with MySQL connection
func NewWorker(totalVouchers int64, totalPods int, db *sql.DB) *Worker {
	w := &Worker{
		TotalVouchers:     totalVouchers,
		TotalPods:         totalPods,
		VoucherDistChan:   VoucherDistributionChannel,
		ActivePods:        make(map[string]*PodInfo),
		DistributedRanges: make(map[string]*VoucherRange),
		CurrentVoucherID:  1,
		DB:                db,
		RedisIndexed:      false,
		LastDistMinute:    -1,
	}

	simCache, err := NewSimulatedCache("worker", PodRegistrationChannel, w.handlePodHeartbeat)
	if err != nil {
		log.Fatalf("[Worker] Cache init failed: %v", err)
	}
	w.Cache = simCache

	// Validate and sync Redis index with MySQL on startup
	w.validateAndSyncRedisIndex()

	return w
}

// validateAndSyncRedisIndex ensures Redis contains valid voucher bytes matching MySQL
func (w *Worker) validateAndSyncRedisIndex() {
	log.Println("[Worker] Validating Redis index against MySQL...")

	// Check if Redis index exists
	indexKey := "voucher-index:meta"
	if meta, ok := w.Cache.globalBus.Get(indexKey); ok {
		var indexMeta struct {
			TotalIndexed int64 `json:"total_indexed"`
			LastSync     int64 `json:"last_sync"`
		}
		if json.Unmarshal(meta, &indexMeta) == nil && indexMeta.TotalIndexed == w.TotalVouchers {
			log.Printf("[Worker] Redis index valid: %d vouchers indexed at %d",
				indexMeta.TotalIndexed, indexMeta.LastSync)
			w.RedisIndexed = true
			return
		}
	}

	// Index missing or mismatch - re-index from MySQL
	log.Println("[Worker] Redis index missing or mismatched, re-indexing from MySQL...")
	w.reindexVouchersToRedis()
}

// reindexVouchersToRedis reads vouchers from MySQL and pre-indexes them as []bytes in Redis
// Stores both individual voucher:{id} keys (for claim validation) and
// voucher-pack:{start}-{end} keys (for bulk distribution fetch).
func (w *Worker) reindexVouchersToRedis() {
	if w.DB == nil {
		log.Println("[Worker] No MySQL connection, using simulated index")
		w.createSimulatedIndex()
		return
	}

	indexed := int64(0)

	// Process in pack-sized chunks aligned to VoucherPackSize
	for packStart := int64(1); packStart <= w.TotalVouchers; packStart += VoucherPackSize {
		packEnd := min(packStart+VoucherPackSize-1, w.TotalVouchers)

		vouchers, err := w.FetchVouchersFromDB(packStart, packEnd, VoucherPackSize)
		if err != nil {
			log.Printf("[Worker] Failed to fetch vouchers %d-%d: %v", packStart, packEnd, err)
			continue
		}

		// Build individual keys + pack
		values := make(map[string][]byte)
		packVouchers := make([][]byte, 0, len(vouchers))

		for _, v := range vouchers {
			// voucherBytes := GenerateVoucherBytes(uint32(v.ID), true, uint32(v.ID)%PINMask)
			voucherBytes := CastDBVoucherBytes(&v)
			key := fmt.Sprintf("voucher:%d", v.ID)
			values[key] = voucherBytes
			packVouchers = append(packVouchers, voucherBytes)
			indexed++
		}

		// Store the pack as a single Redis key
		packData, _ := json.Marshal(packVouchers)
		packKey := fmt.Sprintf("voucher-pack:%d-%d", packStart, packEnd)
		values[packKey] = packData

		w.Cache.globalBus.BulkSet(&values)

		if indexed%(VoucherPackSize*10) == 0 {
			log.Printf("[Worker] Indexed %d/%d vouchers to Redis (%d packs)",
				indexed, w.TotalVouchers, indexed/VoucherPackSize)
		}
	}

	// Store index metadata
	meta := map[string]int64{
		"total_indexed": indexed,
		"last_sync":     time.Now().Unix(),
		"pack_size":     VoucherPackSize,
	}
	metaBytes, _ := json.Marshal(meta)
	w.Cache.globalBus.Set("voucher-index:meta", metaBytes)

	totalPacks := (indexed + VoucherPackSize - 1) / VoucherPackSize
	log.Printf("[Worker] Re-indexed %d vouchers to Redis in %d packs", indexed, totalPacks)
	w.RedisIndexed = true
}

// createSimulatedIndex creates a simulated index when MySQL is not available.
// Pre-generates voucher packs eagerly so distribution uses bulk pack reads.
func (w *Worker) createSimulatedIndex() {
	log.Printf("[Worker] Creating simulated voucher packs (%d vouchers, %d per pack)...",
		w.TotalVouchers, VoucherPackSize)

	start := time.Now()
	packCount := int64(0)

	for packStart := int64(1); packStart <= w.TotalVouchers; packStart += VoucherPackSize {
		packEnd := min(packStart+VoucherPackSize-1, w.TotalVouchers)
		packSize := int(packEnd - packStart + 1)

		packVouchers := make([][]byte, packSize)
		for i := range packSize {
			voucherID := packStart + int64(i)
			pin := rand.Uint32() & PINMask
			packVouchers[i] = GenerateVoucherBytes(uint32(voucherID), true, pin)
		}

		packData, _ := json.Marshal(packVouchers)
		packKey := fmt.Sprintf("voucher-pack:%d-%d", packStart, packEnd)
		w.Cache.globalBus.Set(packKey, packData)
		packCount++

		if packCount%100 == 0 {
			log.Printf("[Worker] Generated %d/%d packs (%s vouchers)",
				packCount, (w.TotalVouchers+VoucherPackSize-1)/VoucherPackSize,
				formatMillions(packStart+int64(packSize)-1))
		}
	}

	// Store index metadata
	meta := map[string]int64{
		"total_indexed": w.TotalVouchers,
		"last_sync":     time.Now().Unix(),
		"pack_size":     VoucherPackSize,
	}
	metaBytes, _ := json.Marshal(meta)
	w.Cache.globalBus.Set("voucher-index:meta", metaBytes)

	log.Printf("[Worker] Simulated index complete: %d packs in %v", packCount, time.Since(start).Round(time.Millisecond))
	w.RedisIndexed = true
}

// handlePodHeartbeat processes heartbeat events from read pods
func (w *Worker) handlePodHeartbeat(event types.InvalidationEvent) any {
	var status map[string]any
	var newPodRegistered bool
	if err := json.Unmarshal(event.Value, &status); err == nil {
		if pid, ok := status["pod_id"].(string); ok {
			w.mu.Lock()
			if info, exists := w.ActivePods[pid]; exists {
				info.LastSeen = time.Now()
				info.IsHealthy = true
			} else {
				var podIdx int
				fmt.Sscanf(pid, "pod-%d", &podIdx)
				vouchersPerPod := w.TotalVouchers / int64(w.TotalPods)
				w.ActivePods[pid] = &PodInfo{
					PodID:     pid,
					PodIndex:  podIdx,
					LastSeen:  time.Now(),
					StartID:   int64(podIdx)*vouchersPerPod + 1,
					EndID:     int64(podIdx+1) * vouchersPerPod,
					IsHealthy: true,
				}
				w.DistributedRanges[pid] = w.loadVoucherRange(pid, podIdx)
				newPodRegistered = true
				log.Printf("[Worker] Registered new pod: %s, range [%d, %d]",
					pid, w.DistributedRanges[pid].StartID, w.DistributedRanges[pid].EndID)
			}
			w.mu.Unlock()
			if newPodRegistered {
				w.persistDistributionProgress()
			}
		}
	}
	return nil
}

// Run starts the worker's main loop
func (w *Worker) Run(ctx context.Context) {
	log.Println("[Worker] Starting voucher distribution worker...")

	// Listen for redemptions (claims)
	redemptionSub := w.Cache.globalBus.Subscribe(RedemptionChannel)
	go w.processRedemptions(ctx, redemptionSub)

	// Distribution ticker (every 5s = prepare for next minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Health check ticker
	healthTicker := time.NewTicker(3 * time.Second)
	defer healthTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[Worker] Shutting down...")
			w.Cache.Close()
			return
		case <-ticker.C:
			w.distributeVouchers()
		case <-healthTicker.C:
			w.checkPodHealth()
		}
	}
}

// processRedemptions handles claim events from read pods
func (w *Worker) processRedemptions(ctx context.Context, sub chan types.InvalidationEvent) {
	claimCount := int64(0)
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-sub:
			if event.Action == types.Set {
				claimCount++
				var claim map[string]any
				if err := json.Unmarshal(event.Value, &claim); err == nil {
					voucherCode, _ := claim["voucher"].(string)
					// isValid, voucherID, _, err := ValidateVoucherCode(voucherCode)
					isValid, _, _, err := ValidateVoucherCode(voucherCode)
					if err == nil && isValid {
						// log.Printf("[Worker] Claim #%d validated: voucher_id=%d", claimCount, voucherID)
					}
				}
			}
		}
	}
}

// checkPodHealth marks pods as unhealthy if no heartbeat received and monitors n Redis health slots for pod status
func (w *Worker) checkPodHealth() {
	now := time.Now()

	// Check all n health slots in Redis
	for i := 0; i < w.TotalPods; i++ {
		slotKey := fmt.Sprintf("pod:health:%d", i)
		data, exists := w.Cache.globalBus.Get(slotKey)

		podID := fmt.Sprintf("pod-%d", i)

		var newPodRegistered bool

		w.mu.Lock()
		info, hasInfo := w.ActivePods[podID]

		if !exists {
			// No health slot data - pod may not have started yet
			if hasInfo && info.IsHealthy {
				if now.Sub(info.LastSeen) > 10*time.Second {
					info.IsHealthy = false
					log.Printf("[Worker] Pod %s marked DEAD (health slot empty for 10s)", podID)
				}
			}
			w.mu.Unlock()
			continue
		}

		var status map[string]any
		if err := json.Unmarshal(data, &status); err != nil {
			w.mu.Unlock()
			continue
		}

		// Check timestamp
		ts, _ := status["timestamp"].(float64)
		lastSeen := time.Unix(int64(ts), 0)

		if hasInfo {
			info.LastSeen = lastSeen
			if now.Sub(lastSeen) > 10*time.Second {
				if info.IsHealthy {
					info.IsHealthy = false
					log.Printf("[Worker] Pod %s marked DEAD (no heartbeat for 10s)", podID)
				}
			} else {
				if !info.IsHealthy {
					info.IsHealthy = true
					log.Printf("[Worker] Pod %s recovered (heartbeat received)", podID)
				}
			}
		} else {
			// Register new pod from health slot
			vouchersPerPod := w.TotalVouchers / int64(w.TotalPods)
			w.ActivePods[podID] = &PodInfo{
				PodID:     podID,
				PodIndex:  i,
				LastSeen:  lastSeen,
				StartID:   int64(i)*vouchersPerPod + 1,
				EndID:     int64(i+1) * vouchersPerPod,
				IsHealthy: true,
			}
			w.DistributedRanges[podID] = w.loadVoucherRange(podID, i)
			newPodRegistered = true
			log.Printf("[Worker] Discovered pod from health slot: %s, range [%d, %d]",
				podID, w.DistributedRanges[podID].StartID, w.DistributedRanges[podID].EndID)
		}
		w.mu.Unlock()

		if newPodRegistered {
			w.persistDistributionProgress()
		}
	}
}

// distributeVouchers sends voucher batches to healthy pods via per-pod channels
// Fetches pre-indexed []bytes from Redis (not generated on-the-fly)
func (w *Worker) distributeVouchers() {
	w.mu.RLock()
	var healthyPods []*PodInfo
	for _, info := range w.ActivePods {
		if info.IsHealthy {
			healthyPods = append(healthyPods, info)
		}
	}
	w.mu.RUnlock()

	if len(healthyPods) == 0 {
		log.Println("[Worker] No healthy pods available for distribution")
		return
	}

	sort.Slice(healthyPods, func(i, j int) bool {
		return healthyPods[i].PodIndex < healthyPods[j].PodIndex
	})

	now := time.Now()
	nextMinute := (now.Minute() + 1) % 60
	batchPerPod := int64(PodServeRate * 60) // Vouchers per pod per minute (50k req/s × 60s = 3M)

	// Wait until 10 seconds into the minute before calculating distribution
	// so read pods have finished migrating their previous fragments
	if now.Second() < 10 {
		return
	}

	w.mu.Lock()
	if w.LastDistMinute == nextMinute {
		w.mu.Unlock()
		return
	}
	w.LastDistMinute = nextMinute
	w.mu.Unlock()

	log.Printf("[Worker] Distributing to %d healthy pods for minute %d (%d vouchers/pod)",
		len(healthyPods), nextMinute, batchPerPod)

	for _, podInfo := range healthyPods {
		// Snapshot range info under lock, then release for Redis fetches
		w.mu.Lock()
		vRange := w.DistributedRanges[podInfo.PodID]
		if vRange == nil {
			w.mu.Unlock()
			continue
		}
		fetchStart := vRange.CurrentID
		vEndID := vRange.EndID
		lastFragCount := vRange.LastFragmentCount
		w.mu.Unlock()

		// Read pod-fragments from Redis to skip fragmented ranges
		var fragments []FragmentRange
		fragKey := fmt.Sprintf("pod-fragments:%s", podInfo.PodID)
		if data, ok := w.Cache.globalBus.Get(fragKey); ok {
			json.Unmarshal(data, &fragments)
		}

		migratedSize := int64(0)
		for i := lastFragCount; i < len(fragments); i++ {
			migratedSize += (fragments[i].End - fragments[i].Start + 1)
		}

		batchSizeToFetch := batchPerPod - migratedSize
		if batchSizeToFetch <= 0 {
			// Fragments cover the whole batch, update fragment count and skip adding new ones
			w.mu.Lock()
			vRange = w.DistributedRanges[podInfo.PodID]
			if vRange != nil {
				vRange.LastFragmentCount = len(fragments)
			}
			w.mu.Unlock()
			continue
		}

		fetchEnd := min(fetchStart+batchSizeToFetch-1, vEndID)
		if fetchStart > fetchEnd {
			continue // Range exhausted
		}

		// Fetch all vouchers for this pod using pack-based bulk retrieval
		allVouchers := w.fetchVouchersFromPacks(fetchStart, fetchEnd)
		if len(allVouchers) == 0 {
			continue
		}

		// Chunk into sub-batches of PodServeRate (50K) for pub/sub delivery
		subBatchSize := PodServeRate
		totalSent := 0

		for off := 0; off < len(allVouchers); off += subBatchSize {
			end := min(off+subBatchSize, len(allVouchers))
			chunk := allVouchers[off:end]

			batch := VoucherBatch{
				BatchID:       fmt.Sprintf("batch-%d-%s-%d", nextMinute, podInfo.PodID, time.Now().UnixNano()),
				AssignedPodID: podInfo.PodID,
				MinuteKey:     nextMinute,
				Vouchers:      chunk,
				StartIdx:      fetchStart + int64(off),
			}

			data, _ := json.Marshal(batch)

			// Publish to per-pod channel (each pod has its own channel)
			podChannel := fmt.Sprintf("%s-%d", VoucherDistributionChannel, podInfo.PodIndex)
			w.Cache.globalBus.Publish(podChannel, types.InvalidationEvent{
				Key:    "batch:" + batch.BatchID,
				Value:  data,
				Sender: "worker",
				Action: types.Set,
			})

			totalSent += len(chunk)
		}

		// Update range progress under lock
		fetchCount := int64(fetchEnd - fetchStart + 1)
		w.mu.Lock()
		vRange = w.DistributedRanges[podInfo.PodID]
		if vRange != nil {
			vRange.CurrentID = fetchEnd + 1
			vRange.Distributed += fetchCount
			vRange.LastFragmentCount = len(fragments)
		}
		w.mu.Unlock()

		log.Printf("[Worker] Sent %d vouchers to %s in %d sub-batches for minute %d (skipped %d fragments)",
			totalSent, podInfo.PodID, (int(fetchCount)+PodServeRate-1)/PodServeRate, nextMinute, migratedSize)
	}

	// Persist distribution range progress to Redis for monitoring
	w.persistDistributionProgress()
}

// persistDistributionProgress saves current distribution ranges to Redis for the monitor
func (w *Worker) persistDistributionProgress() {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for podID, vRange := range w.DistributedRanges {
		rangeData := map[string]int64{
			"start_id":            vRange.StartID,
			"end_id":              vRange.EndID,
			"current_id":          vRange.CurrentID,
			"distributed":         vRange.Distributed,
			"last_fragment_count": int64(vRange.LastFragmentCount),
		}
		data, _ := json.Marshal(rangeData)
		w.Cache.globalBus.Set(fmt.Sprintf("dist-range:%s", podID), data)
	}
}

// loadVoucherRange loads or initializes a VoucherRange for a pod
func (w *Worker) loadVoucherRange(podID string, podIndex int) *VoucherRange {
	vouchersPerPod := w.TotalVouchers / int64(w.TotalPods)
	vr := &VoucherRange{
		StartID:   int64(podIndex)*vouchersPerPod + 1,
		EndID:     int64(podIndex+1) * vouchersPerPod,
		CurrentID: int64(podIndex)*vouchersPerPod + 1,
	}

	distKey := fmt.Sprintf("dist-range:%s", podID)
	if data, ok := w.Cache.globalBus.Get(distKey); ok {
		var state map[string]int64
		if err := json.Unmarshal(data, &state); err == nil {
			if cur, ok := state["current_id"]; ok && cur > 0 {
				vr.CurrentID = cur
			}
			if dist, ok := state["distributed"]; ok {
				vr.Distributed = dist
			}
			if lfc, ok := state["last_fragment_count"]; ok {
				vr.LastFragmentCount = int(lfc)
			}
		}
	}
	return vr
}

// fetchVouchersFromPacks loads voucher bytes for a range [startID, endID] using
// packed Redis keys (voucher-pack:{start}-{end}), reducing Redis round-trips by ~50,000x.
// Falls back to individual key lookup for any packs not found.
func (w *Worker) fetchVouchersFromPacks(startID, endID int64) [][]byte {
	total := int(endID - startID + 1)
	result := make([][]byte, 0, total)

	// Iterate through all packs that overlap with [startID, endID]
	// Packs are aligned: [1, PackSize], [PackSize+1, 2*PackSize], ...
	for id := startID; id <= endID; {
		// Calculate which pack this ID belongs to
		packIdx := (id - 1) / VoucherPackSize // 0-based pack index
		packStart := packIdx*VoucherPackSize + 1
		packEnd := min(packStart+VoucherPackSize-1, w.TotalVouchers)

		// Load pack from Redis
		packKey := fmt.Sprintf("voucher-pack:%d-%d", packStart, packEnd)
		packData, ok := w.Cache.globalBus.Get(packKey)

		if !ok {
			// Pack not found — fallback to individual keys for this range
			chunkEnd := min(packEnd, endID)
			for vid := id; vid <= chunkEnd; vid++ {
				vb := w.getVoucherBytesFromRedis(vid)
				if vb != nil {
					result = append(result, vb)
				}
			}
			id = chunkEnd + 1
			continue
		}

		// Unmarshal the pack
		var packVouchers [][]byte
		if err := json.Unmarshal(packData, &packVouchers); err != nil {
			log.Printf("[Worker] Failed to unmarshal pack %s: %v", packKey, err)
			id = min(packEnd, endID) + 1
			continue
		}

		// Slice: extract only the requested sub-range within this pack
		sliceStart := int(id - packStart)                    // offset within the pack
		sliceEnd := int(min(endID, packEnd) - packStart + 1) // exclusive end
		if sliceStart < len(packVouchers) {
			if sliceEnd > len(packVouchers) {
				sliceEnd = len(packVouchers)
			}
			result = append(result, packVouchers[sliceStart:sliceEnd]...)
		}

		id = min(packEnd, endID) + 1
	}

	return result
}

// getVoucherBytesFromRedis fetches a single voucher by individual key.
// Used as fallback when packs are unavailable and for claim validation.
func (w *Worker) getVoucherBytesFromRedis(voucherID int64) []byte {
	key := fmt.Sprintf("voucher:%d", voucherID)

	// Try Redis first (pre-indexed bytes)
	if data, ok := w.Cache.globalBus.Get(key); ok {
		return data
	}

	// Fallback: Generate on-the-fly for simulation mode
	if !w.RedisIndexed {
		// pin := rand.Uint32() & PINMask
		// voucherBytes := GenerateVoucherBytes(uint32(voucherID), true, pin)
		// w.Cache.globalBus.Set(key, voucherBytes)
		// return voucherBytes
		panic("Voucher not found in Redis and re-indexing is not supported in this version")
	}

	log.Printf("[Worker] Voucher %d not found in Redis, querying MySQL...", voucherID)

	// Query MySQL and re-index
	if w.DB != nil {
		vouchers, err := w.FetchVouchersFromDB(voucherID, voucherID, 1)
		if err == nil && len(vouchers) > 0 {
			v := vouchers[0]
			// voucherBytes := GenerateVoucherBytes(uint32(v.ID), v.IsValid, uint32(v.ID)%PINMask)
			voucherBytes := CastDBVoucherBytes(&v)
			w.Cache.globalBus.Set(key, voucherBytes)
			return voucherBytes
		}
	}

	return nil
}

// FetchVouchersFromDB queries vouchers from MySQL within a range (for real implementation)
func (w *Worker) FetchVouchersFromDB(startID, endID, limit int64) ([]DBVoucher, error) {
	if w.DB == nil {
		return nil, nil // Simulated mode
	}

	query := "SELECT id, voucher_code, is_valid FROM voucher WHERE id >= ? AND id <= ? LIMIT ?"
	rows, err := w.DB.Query(query, startID, endID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vouchers []DBVoucher
	for rows.Next() {
		var v DBVoucher
		if err := rows.Scan(&v.ID, &v.Code, &v.IsValid); err != nil {
			continue
		}
		vouchers = append(vouchers, v)
	}
	return vouchers, nil
}
