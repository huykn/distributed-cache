package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huykn/distributed-cache/types"
)

const (
	ClaimQueueSize      = 50_0000                  // Large enough to avoid dropping claims at high throughput
	SpinHistoryBulkSize = 1000                     // Bulk write spin history to Redis every N spins
	bufCap              = int64(PodServeRate * 60) // Exact capacity for one minute
)

// SpinResult represents a spin that will be written to Redis for user history
type SpinResult struct {
	UserID      string
	VoucherCode string
	VoucherID   uint32
	IsValid     bool
	Timestamp   int64
}

// ClaimRequest represents a pending claim in the queue
type ClaimRequest struct {
	UserID      string
	VoucherCode string
	VoucherID   uint32
	Timestamp   int64
}

// FragmentRange tracks fragmented voucher ID ranges for recovery
type FragmentRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// MinuteStore holds vouchers for a specific minute (lock-free access)
type MinuteStore struct {
	Vouchers      []atomic.Value // Each holds []byte - lock-free read
	CurrentIdx    int64          // Atomic index for pop operation
	Total         int64          // Total vouchers in this minute
	StartID       int64          // First voucher ID in this store
	MigratedCount int64          // Tracking: vouchers migrated manually
	ReceivedCount int64          // Tracking: vouchers received from worker
}

func (p *MinuteStore) reset() {
	p.Vouchers = make([]atomic.Value, bufCap)
	p.CurrentIdx = 0
	p.Total = 0
	p.StartID = 0
	p.MigratedCount = 0
	p.ReceivedCount = 0
}

// ReadPod represents a read pod instance with lock-free voucher serving
type ReadPod struct {
	ID            string
	PodIndex      int
	TotalPods     int
	TotalVouchers int64
	StartID       int64 // Assigned voucher range start
	EndID         int64 // Assigned voucher range end

	// Storage: [60] for each minute, lock-free access
	MinuteStores [60]MinuteStore

	// In-memory claim queue for async processing
	ClaimQueue chan ClaimRequest

	// In-memory spin history queue - bulk write to Redis for user paging
	SpinHistoryQueue chan SpinResult
	spinHistoryBatch []SpinResult
	spinHistoryMu    sync.Mutex

	// Fragment tracking for minute migration
	Fragments  []FragmentRange
	fragmentMu sync.Mutex

	// Stats (atomic for lock-free access)
	SpinCount  int64
	ClaimCount int64

	Cache     *SimulatedCache
	IsHealthy int32 // Atomic: 1 = healthy, 0 = unhealthy
	stopCh    chan struct{}
	killed    int32 // Atomic: 1 = killed

	// Synchronization for clean shutdown: wait for claim queue drain before saving state
	claimDone chan struct{}
}

// NewReadPod creates a new ReadPod with assigned voucher range
func NewReadPod(podIndex, totalPods int, totalVouchers, startID, endID int64) *ReadPod {
	id := fmt.Sprintf("pod-%d", podIndex)
	pod := &ReadPod{
		ID:               id,
		PodIndex:         podIndex,
		TotalPods:        totalPods,
		TotalVouchers:    totalVouchers,
		StartID:          startID,
		EndID:            endID,
		IsHealthy:        1,
		stopCh:           make(chan struct{}),
		ClaimQueue:       make(chan ClaimRequest, ClaimQueueSize),
		SpinHistoryQueue: make(chan SpinResult, ClaimQueueSize),
		spinHistoryBatch: make([]SpinResult, 0, SpinHistoryBulkSize),
		Fragments:        make([]FragmentRange, 0),
		claimDone:        make(chan struct{}),
	}

	// Initialize minute stores
	for i := range 60 {
		pod.MinuteStores[i] = MinuteStore{
			Vouchers:   make([]atomic.Value, bufCap),
			CurrentIdx: 0,
			Total:      0,
		}
	}

	// Create simulated distributed cache with per-pod channel
	podChannel := fmt.Sprintf("%s-%d", VoucherDistributionChannel, podIndex)
	simCache, err := NewSimulatedCache(id, podChannel, pod.handleInvalidation)
	if err != nil {
		log.Fatalf("Failed to create cache for pod %s: %v", id, err)
	}
	pod.Cache = simCache

	return pod
}

// Run starts the pod's background tasks
func (p *ReadPod) Run(ctx context.Context) {
	log.Printf("[%s] Starting Read Pod (range: %d-%d)...", p.ID, p.StartID, p.EndID)

	// Start claim queue processor
	go p.processClaimQueue(ctx)

	// Start spin history bulk writer (writes to Redis for user paging)
	go p.spinHistoryBulkWriter(ctx)

	// Start minute migration worker
	go p.minuteMigrationWorker(ctx)

	// Heartbeat loop - writes to Redis health slot
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			case <-ticker.C:
				if atomic.LoadInt32(&p.killed) == 1 {
					return
				}
				if atomic.LoadInt32(&p.IsHealthy) == 1 {
					p.updateHealthSlot()
				}
			}
		}
	}()

	// Stats reporter
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			case <-ticker.C:
				if atomic.LoadInt32(&p.killed) == 1 {
					return
				}
				spins := atomic.LoadInt64(&p.SpinCount)
				claims := atomic.LoadInt64(&p.ClaimCount)
				log.Printf("[%s] Stats: spins=%d, claims=%d, spinHistoryQueue=%d",
					p.ID, spins, claims, len(p.SpinHistoryQueue))
			}
		}
	}()

	// Wait for context done or kill signal
	select {
	case <-ctx.Done():
	case <-p.stopCh:
	}

	// Wait for claim queue to fully drain before saving state
	// This ensures SaveStateToRedis captures the final ClaimCount
	<-p.claimDone

	// Final drain: catch straggler claims from in-flight HandleSpin goroutines that enqueued after processClaimQueue exited
	for len(p.ClaimQueue) > 0 {
		claim := <-p.ClaimQueue
		p.processSingleClaim(claim)
	}

	p.SaveStateToRedis()
	p.Cache.Close()
}

// Kill stops the pod (simulates pod being killed)
func (p *ReadPod) Kill() {
	atomic.StoreInt32(&p.killed, 1)
	atomic.StoreInt32(&p.IsHealthy, 0)
	close(p.stopCh)
	log.Printf("[%s] Pod killed", p.ID)
}

// spinHistoryBulkWriter collects spin results and bulk writes to Redis for user paging
func (p *ReadPod) spinHistoryBulkWriter(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second) // Flush every second
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.flushSpinHistory()
			return
		case <-p.stopCh:
			p.flushSpinHistory()
			return
		case spin := <-p.SpinHistoryQueue:
			p.spinHistoryMu.Lock()
			p.spinHistoryBatch = append(p.spinHistoryBatch, spin)
			shouldFlush := len(p.spinHistoryBatch) >= SpinHistoryBulkSize
			p.spinHistoryMu.Unlock()
			if shouldFlush {
				p.flushSpinHistory()
			}
		case <-ticker.C:
			p.flushSpinHistory()
		}
	}
}

var poolHistory = sync.Pool{
	New: func() any {
		return make([]SpinResult, 0, SpinHistoryBulkSize)
	},
}

// flushSpinHistory bulk writes spin history to Redis for user paging
func (p *ReadPod) flushSpinHistory() {
	p.spinHistoryMu.Lock()
	if len(p.spinHistoryBatch) == 0 {
		p.spinHistoryMu.Unlock()
		return
	}
	batch := p.spinHistoryBatch
	p.spinHistoryBatch = make([]SpinResult, 0, SpinHistoryBulkSize)
	p.spinHistoryMu.Unlock()

	// Group by user for bulk write (extract VoucherID here, off hot path)
	userSpins := make(map[string][]SpinResult)
	for i, spin := range batch {
		// Extract VoucherID off hot path (deferred from HandleSpin for zero-allocation spin)
		if spin.VoucherCode != "" && spin.VoucherID == 0 {
			_, vid, _, _ := ValidateVoucherCode(spin.VoucherCode)
			batch[i].VoucherID = vid
		}
		userSpins[spin.UserID] = append(userSpins[spin.UserID], batch[i])
	}

	// Write to Redis (simulated: store as list per user)
	for userID, spins := range userSpins {
		key := fmt.Sprintf("user:%s:spins", userID)
		// Append to existing history
		var history []SpinResult
		if data, ok := p.Cache.globalBus.Get(key); ok {
			history = poolHistory.Get().([]SpinResult)
			json.Unmarshal(data, &history)
		}
		history = append(history, spins...)
		data, _ := json.Marshal(history)
		p.Cache.globalBus.Set(key, data)
		poolHistory.Put(history)
	}

	// Only log flush periodically to reduce noise at high throughput
	spins := atomic.LoadInt64(&p.SpinCount)
	if spins%50000 < int64(len(batch)) {
		// log.Printf("[%s] Flushed %d spin results to Redis (%d users, total spins: %d)", p.ID, len(batch), len(userSpins), spins)
	}
}

// updateHealthSlot writes heartbeat to Redis health slot (n slots for n pods)
func (p *ReadPod) updateHealthSlot() {
	slotKey := fmt.Sprintf("pod:health:%d", p.PodIndex)
	currentMin := time.Now().Minute()
	nextMin := (currentMin + 1) % 60
	status := map[string]any{
		"pod_id":            p.ID,
		"pod_index":         p.PodIndex,
		"timestamp":         time.Now().Unix(),
		"is_healthy":        true,
		"start_id":          p.StartID,
		"end_id":            p.EndID,
		"spins":             atomic.LoadInt64(&p.SpinCount),
		"claims":            atomic.LoadInt64(&p.ClaimCount),
		"current_min":       currentMin,
		"current_min_total": atomic.LoadInt64(&p.MinuteStores[currentMin].Total),
		"current_min_idx":   atomic.LoadInt64(&p.MinuteStores[currentMin].CurrentIdx),
		"next_min_total":    atomic.LoadInt64(&p.MinuteStores[nextMin].Total),
		"next_min_idx":      atomic.LoadInt64(&p.MinuteStores[nextMin].CurrentIdx),
	}
	data, _ := json.Marshal(status)
	p.Cache.globalBus.Set(slotKey, data)

	// Also publish for worker to receive
	p.Cache.globalBus.Publish(PodRegistrationChannel, types.InvalidationEvent{
		Key:    slotKey,
		Value:  data,
		Sender: p.ID,
		Action: types.Set,
	})
}

// processClaimQueue handles async claim validation and Redis writes
func (p *ReadPod) processClaimQueue(ctx context.Context) {
	defer close(p.claimDone) // Signal that claim queue is fully drained
	for {
		select {
		case <-ctx.Done():
			// Drain remaining claims before shutdown
			for len(p.ClaimQueue) > 0 {
				claim := <-p.ClaimQueue
				p.processSingleClaim(claim)
			}
			return
		case <-p.stopCh:
			// Pod killed — drain remaining claims before exit so SaveStateToRedis captures final count
			for len(p.ClaimQueue) > 0 {
				claim := <-p.ClaimQueue
				p.processSingleClaim(claim)
			}
			return
		case claim := <-p.ClaimQueue:
			p.processSingleClaim(claim)
		}
	}
}

func (p *ReadPod) processSingleClaim(claim ClaimRequest) {
	// Stateless validation using snowflake ID (no DB query needed)
	isValid, voucherID, _, err := ValidateVoucherCode(claim.VoucherCode)
	if err != nil {
		log.Printf("[%s] Invalid voucher code: %s", p.ID, claim.VoucherCode)
		return
	}

	if !isValid {
		log.Printf("[%s] Voucher marked invalid: %d", p.ID, voucherID)
		return
	}

	// Validate voucher belongs to this pod's range
	if !ValidateVoucherForPod(voucherID, p.PodIndex, p.TotalPods, p.TotalVouchers) {
		log.Printf("[%s] Voucher %d not in pod range [%d, %d]",
			p.ID, voucherID, p.StartID, p.EndID)
		return
	}

	// Send to worker for Redis/MySQL persistence
	event := map[string]any{
		"user_id":    claim.UserID,
		"voucher":    claim.VoucherCode,
		"voucher_id": voucherID,
		"pod_id":     p.ID,
		"ts":         claim.Timestamp,
		"validated":  true,
	}
	data, _ := json.Marshal(event)

	p.Cache.globalBus.Publish(RedemptionChannel, types.InvalidationEvent{
		Key:    fmt.Sprintf("claim:%s:%d", claim.UserID, voucherID),
		Value:  data,
		Sender: p.ID,
		Action: types.Set,
	})

	atomic.AddInt64(&p.ClaimCount, 1)
}

// minuteMigrationWorker compacts unconsumed vouchers in the unified buffer periodically.
// This prevents voucher loss when new batches arrive and the buffer would otherwise overflow.
func (p *ReadPod) minuteMigrationWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	lastMigratedMin := -1

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			now := time.Now()
			// Run migration right after minute transitions to give worker time to see fragments
			if now.Second() >= 2 && now.Second() <= 5 && now.Minute() != lastMigratedMin {
				p.migrateOldMinuteData()
				lastMigratedMin = now.Minute()
			}
		}
	}
}

// migrateOldMinuteData compacts unused vouchers from previous minutes and moves them to future minutes.
// This ensures fragmented (unconsumed) vouchers are preserved across distribution cycles.
func (p *ReadPod) migrateOldMinuteData() {
	now := time.Now()
	prevMin := (now.Minute() - 1 + 60) % 60
	targetMin := (now.Minute() + 1) % 60

	storePrev := &p.MinuteStores[prevMin]
	storeTarget := &p.MinuteStores[targetMin]

	idx := atomic.LoadInt64(&storePrev.CurrentIdx)
	size := atomic.LoadInt64(&storePrev.Total)

	// Nothing to compact: no consumed vouchers at the front
	if size == 0 || idx >= size {
		atomic.StoreInt64(&storePrev.CurrentIdx, 0)
		atomic.StoreInt64(&storePrev.Total, 0)
		return
	}

	unconsumed := size - idx

	log.Printf("[%s] Migrating %d unused vouchers from minute %d to minute %d",
		p.ID, unconsumed, prevMin, targetMin)

	// Track fragment for recovery
	p.fragmentMu.Lock()
	p.Fragments = append(p.Fragments, FragmentRange{
		Start: storePrev.StartID + idx,
		End:   storePrev.StartID + size - 1,
	})
	fragmentData, _ := json.Marshal(p.Fragments)
	p.fragmentMu.Unlock()
	p.Cache.globalBus.Set(fmt.Sprintf("pod-fragments:%s", p.ID), fragmentData)

	// Move unconsumed vouchers to target minute store
	targetWriteStart := atomic.LoadInt64(&storeTarget.Total)
	bufCap := int64(len(storeTarget.Vouchers))

	validCount := int64(0)
	for i := range unconsumed {
		val := storePrev.Vouchers[idx+i].Load()
		if val != nil {
			if vBytes, ok := val.([]byte); ok && len(vBytes) > 0 {
				pos := targetWriteStart + validCount
				if pos < bufCap {
					storeTarget.Vouchers[pos].Store(vBytes)
					validCount++
				}
			}
		}
		// Clear old to help GC
		storePrev.Vouchers[idx+i].Store([]byte{})
	}

	atomic.AddInt64(&storeTarget.MigratedCount, validCount)
	atomic.StoreInt64(&storeTarget.Total, targetWriteStart+validCount)
	if targetWriteStart == 0 {
		storeTarget.StartID = storePrev.StartID + idx
	}

	// Reset prev minute store
	atomic.StoreInt64(&storePrev.CurrentIdx, 0)
	atomic.StoreInt64(&storePrev.Total, 0)
	atomic.StoreInt64(&storePrev.MigratedCount, 0)
	atomic.StoreInt64(&storePrev.ReceivedCount, 0)
	storePrev.reset()

	log.Printf("[%s] Migration complete: %d vouchers moved to minute %d (total migrated: %d, received: %d, total: %d)",
		p.ID, unconsumed, targetMin, atomic.LoadInt64(&storeTarget.MigratedCount), atomic.LoadInt64(&storeTarget.ReceivedCount), atomic.LoadInt64(&storeTarget.Total))
}

// handleInvalidation is the callback for receiving voucher batches
func (p *ReadPod) handleInvalidation(event types.InvalidationEvent) any {
	if event.Action != types.Set {
		return nil
	}

	batch := poolVoucherBatch.Get().(*VoucherBatch)
	defer poolVoucherBatch.Put(batch)
	if err := json.Unmarshal(event.Value, &batch); err != nil {
		return nil
	}

	// Filter: Only process if assigned to this pod
	if batch.AssignedPodID != "" && batch.AssignedPodID != p.ID {
		return nil
	}

	minuteKey := batch.MinuteKey
	store := &p.MinuteStores[minuteKey]

	writeStart := atomic.LoadInt64(&store.Total)
	bufCap := int64(len(store.Vouchers))

	// If this is the first write to this minute store, set StartID
	if writeStart == 0 {
		store.StartID = batch.StartIdx
	}

	// Write to the minute's store without mutex (worker ensures single writer per pod/minute)
	validCount := int64(0)
	for _, v := range batch.Vouchers {
		if len(v) > 0 {
			pos := writeStart + validCount
			if pos < bufCap {
				store.Vouchers[pos].Store(v)
				validCount++
			}
		}
	}

	atomic.AddInt64(&store.ReceivedCount, validCount)
	newSize := min(writeStart+validCount, bufCap)
	atomic.StoreInt64(&store.Total, newSize)

	log.Printf("[%s] Loaded %d vouchers from worker to minute %d (total migrated: %d, received: %d, total: %d/%d, startID: %d)",
		p.ID, validCount, minuteKey, atomic.LoadInt64(&store.MigratedCount), atomic.LoadInt64(&store.ReceivedCount), newSize, int64(PodServeRate*60), batch.StartIdx)

	return &batch
}

// HandleSpin - lock-free voucher pop operation (hot path, zero allocation)
// Flow: pop from unified buffer -> add to spin history queue (memory) -> return voucher to user
// Background: spin history queue -> bulk write to Redis for user paging
func (p *ReadPod) HandleSpin(userID string) (voucherCode string, isValid, ok bool) {
	// Lock-free health check
	if atomic.LoadInt32(&p.IsHealthy) == 0 || atomic.LoadInt32(&p.killed) == 1 {
		return "", false, false
	}

	// Claim next voucher from current minute store using lock-free AddInt64
	currentMin := time.Now().Minute()
	store := &p.MinuteStores[currentMin]

	idx := atomic.AddInt64(&store.CurrentIdx, 1) - 1
	size := atomic.LoadInt64(&store.Total)

	if idx >= size {
		// No vouchers available yet, but we already incremented the index.
		// That's fine, subsequent vouchers loaded by worker or migrations
		// will be appended to Total, so we just miss a spin but index is safe.
		// For strictness, if idx exceeds capacity, it's safer to not read.
		return "", false, false
	}

	val := store.Vouchers[idx].Load()
	if val == nil {
		return "", false, false
	}
	voucherBytes := val.([]byte)

	// D-8: Increment SpinCount only after successful voucher fetch
	atomic.AddInt64(&p.SpinCount, 1)

	// Parse voucher (no unmarshal needed — zero-serialization)
	code, valid := ParseVoucherBytes(voucherBytes)

	// Add to spin history queue (internal memory queue, not Redis)
	select {
	case p.SpinHistoryQueue <- SpinResult{
		UserID:      userID,
		VoucherCode: code,
		IsValid:     valid,
		Timestamp:   time.Now().UnixNano(),
	}:
	default:
	}

	// D-2: Enqueue to ClaimQueue for background pre-validation
	// Uses select with stopCh to avoid blocking forever if queue processor already exited
	if valid {
		select {
		case p.ClaimQueue <- ClaimRequest{
			UserID:      userID,
			VoucherCode: code,
			Timestamp:   time.Now().Unix(),
		}:
		case <-p.stopCh:
			// Pod is shutting down — count claim directly since queue processor may be gone
			atomic.AddInt64(&p.ClaimCount, 1)
		}
	}

	return code, valid, true
}

// HandleClaim - user confirms voucher is valid and redeems it
// Flow: decode snowflake (check is_valid) -> check Redis (exists, is_valid=1, same user_id) -> return 200/400
func (p *ReadPod) HandleClaim(userID, voucherCode string) (success bool, errMsg string) {
	if atomic.LoadInt32(&p.IsHealthy) == 0 || atomic.LoadInt32(&p.killed) == 1 {
		return false, "pod unhealthy"
	}

	// Step 1: Decode snowflake to check is_valid first (stateless check)
	isValid, voucherID, _, err := ValidateVoucherCode(voucherCode)
	if err != nil {
		return false, fmt.Sprintf("invalid voucher code: %v", err)
	}
	if !isValid {
		return false, "voucher is marked invalid"
	}

	// Step 2: Check Redis for voucher existence and user ownership
	voucherKey := fmt.Sprintf("voucher:%d", voucherID)
	voucherData, exists := p.Cache.globalBus.Get(voucherKey)
	if !exists {
		return false, "voucher not found in Redis"
	}

	// Parse voucher bytes from Redis to double-check is_valid
	_, redisIsValid := ParseVoucherBytes(voucherData)
	if !redisIsValid {
		return false, "voucher is invalid in Redis"
	}

	// Step 3: Check if voucher was already claimed or belongs to user
	claimKey := fmt.Sprintf("claim:%d", voucherID)
	if claimData, claimed := p.Cache.globalBus.Get(claimKey); claimed {
		var existingClaim map[string]any
		if json.Unmarshal(claimData, &existingClaim) == nil {
			if claimedBy, ok := existingClaim["user_id"].(string); ok && claimedBy != userID {
				return false, "voucher already claimed by another user"
			}
		}
	}

	// Step 4: Mark voucher as claimed in Redis
	claimRecord := map[string]any{
		"user_id":    userID,
		"voucher_id": voucherID,
		"claimed_at": time.Now().Unix(),
		"pod_id":     p.ID,
	}
	claimBytes, _ := json.Marshal(claimRecord)
	p.Cache.globalBus.Set(claimKey, claimBytes)

	atomic.AddInt64(&p.ClaimCount, 1)

	// Publish claim event for worker
	p.Cache.globalBus.Publish(RedemptionChannel, types.InvalidationEvent{
		Key:    claimKey,
		Value:  claimBytes,
		Sender: p.ID,
		Action: types.Set,
	})

	return true, ""
}

// SaveStateToRedis persists pod state for recovery after restart
func (p *ReadPod) SaveStateToRedis() {
	log.Printf("[%s] Saving state to Redis...", p.ID)

	// Flush remaining spin history
	p.flushSpinHistory()

	state := map[string]any{
		"pod_id":      p.ID,
		"pod_index":   p.PodIndex,
		"start_id":    p.StartID,
		"end_id":      p.EndID,
		"spin_count":  atomic.LoadInt64(&p.SpinCount),
		"claim_count": atomic.LoadInt64(&p.ClaimCount),
		"fragments":   p.Fragments,
		"timestamp":   time.Now().Unix(),
	}

	state["current_min"] = time.Now().Minute()

	data, _ := json.Marshal(state)
	p.Cache.globalBus.Set(fmt.Sprintf("pod-state:%s", p.ID), data)

	log.Printf("[%s] State saved: spins=%d, claims=%d, fragments=%d",
		p.ID, state["spin_count"], state["claim_count"], len(p.Fragments))
}

// RestoreStateFromRedis restores pod state after restart (new pod taking over)
func (p *ReadPod) RestoreStateFromRedis() {
	stateKey := fmt.Sprintf("pod-state:%s", p.ID)
	data, exists := p.Cache.globalBus.Get(stateKey)
	if !exists {
		log.Printf("[%s] No saved state in Redis, starting fresh", p.ID)
		return
	}

	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("[%s] Failed to parse saved state: %v", p.ID, err)
		return
	}

	// Restore stats
	if spinCount, ok := state["spin_count"].(float64); ok {
		atomic.StoreInt64(&p.SpinCount, int64(spinCount))
	}
	if claimCount, ok := state["claim_count"].(float64); ok {
		atomic.StoreInt64(&p.ClaimCount, int64(claimCount))
	}

	// Restore voucher buffer: re-load ALL unconsumed vouchers from Redis index
	// This includes:
	// (1) vouchers in old buffer not yet consumed, AND
	// (2) vouchers distributed by worker via pub/sub that were LOST during pod downtime
	//
	// Note: Use SpinCount (cumulative across ALL incarnations) to determine total vouchers consumed,
	// NOT oldBufIdx which only tracks the current incarnation's buffer consumption and gets reset to 0 on each restore.

	// Check worker's distribution progress to recover lost pub/sub vouchers
	globalBus := GetGlobalBus()
	totalDistributed := int64(0)
	distKey := fmt.Sprintf("dist-range:%s", p.ID)
	if distData, ok := globalBus.Get(distKey); ok {
		var distRange map[string]int64
		if json.Unmarshal(distData, &distRange) == nil {
			workerCurrentID := distRange["current_id"]
			if workerCurrentID > 0 {
				totalDistributed = workerCurrentID - p.StartID
			}
		}
	}

	// SpinCount is cumulative across all incarnations — it tells us exactly how many vouchers have been consumed in total (each successful spin = 1 voucher consumed)
	totalConsumed := atomic.LoadInt64(&p.SpinCount)
	remaining := totalDistributed - totalConsumed
	if remaining > 0 {
		startID := p.StartID + totalConsumed
		endID := p.StartID + totalDistributed - 1

		vouchers := make([][]byte, 0, remaining)

		for id := startID; id <= endID; {
			packIdx := (id - 1) / 50000 // VoucherPackSize
			packStart := packIdx*50000 + 1
			packEnd := min(int64(packStart+50000-1), p.TotalVouchers)

			packKey := fmt.Sprintf("voucher-pack:%d-%d", packStart, packEnd)
			packData, ok := globalBus.Get(packKey)

			chunkEnd := min(packEnd, endID)

			if ok {
				var packVouchers [][]byte
				if err := json.Unmarshal(packData, &packVouchers); err == nil {
					sliceStart := int(id - packStart)
					sliceEnd := int(chunkEnd - packStart + 1)
					if sliceStart < len(packVouchers) {
						if sliceEnd > len(packVouchers) {
							sliceEnd = len(packVouchers)
						}
						vouchers = append(vouchers, packVouchers[sliceStart:sliceEnd]...)
					}
				}
			} else {
				// Fallback to individual keys
				for vid := id; vid <= chunkEnd; vid++ {
					key := fmt.Sprintf("voucher:%d", vid)
					if voucherBytes, ok := globalBus.Get(key); ok {
						vouchers = append(vouchers, voucherBytes)
					}
				}
			}
			id = chunkEnd + 1
		}

		loaded := int64(0)
		for len(vouchers) > 0 {
			// Distribute recovered vouchers across minute stores starting from the current minute
			minOffset := int(loaded / int64(PodServeRate*60))
			if minOffset >= 60 {
				break
			}
			targetMin := (time.Now().Minute() + minOffset) % 60
			store := &p.MinuteStores[targetMin]

			bufCap := int64(len(store.Vouchers))
			availableSpace := bufCap - atomic.LoadInt64(&store.Total)

			if availableSpace <= 0 {
				loaded += int64(PodServeRate * 60)
				continue
			}

			chunkSize := min(int64(len(vouchers)), availableSpace)

			currentTotal := atomic.LoadInt64(&store.Total)
			if currentTotal == 0 {
				store.StartID = startID + loaded
			}

			for i := range chunkSize {
				store.Vouchers[currentTotal+i].Store(vouchers[i])
			}
			atomic.StoreInt64(&store.Total, currentTotal+chunkSize)
			atomic.StoreInt64(&store.CurrentIdx, 0)

			loaded += chunkSize
			vouchers = vouchers[chunkSize:]
		}

		log.Printf("[%s] Restored %d remaining vouchers from Redis index packs (totalConsumed=%d, workerDistributed=%d)",
			p.ID, loaded, totalConsumed, totalDistributed)
	} else {
		// Nothing to load
	}

	// Restore fragment ranges for recovery
	if fragments, ok := state["fragments"].([]any); ok {
		p.fragmentMu.Lock()
		for _, f := range fragments {
			if fm, ok := f.(map[string]any); ok {
				start, startOk := fm["start"].(float64)
				end, endOk := fm["end"].(float64)
				if startOk && endOk {
					p.Fragments = append(p.Fragments, FragmentRange{
						Start: int64(start),
						End:   int64(end),
					})
				}
			}
		}
		p.fragmentMu.Unlock()
	}

	log.Printf("[%s] Restored state from Redis: spins=%d, claims=%d, fragments=%d",
		p.ID, atomic.LoadInt64(&p.SpinCount), atomic.LoadInt64(&p.ClaimCount), len(p.Fragments))
}
