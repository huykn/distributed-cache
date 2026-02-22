package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	ValidBitShift     = 63
	VoucherIDBitShift = 31
	PINMask           = 0x7FFFFFFF
	VoucherIDMask     = 0xFFFFFFFF
)

func main() {
	// Parse flags
	totalRecords := flag.Int64("total-records", 30_000_000, "Total voucher records to insert")
	validPercentage := flag.Int("valid-percentage", 5, "Percentage of vouchers marked as valid (0-100)")
	dsn := flag.String("dsn", "root:123456@tcp(localhost:3307)/voucher_db", "MySQL DSN")
	batchSize := flag.Int("batch-size", 10000, "Records per INSERT batch")
	dropTable := flag.Bool("drop", false, "Drop and recreate the voucher table before seeding")
	flag.Parse()

	log.Printf("Seed Config: total=%d, valid%%=%d, batch=%d, dsn=%s",
		*totalRecords, *validPercentage, *batchSize, *dsn)

	// Connect to MySQL
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	// Set connection pool for bulk inserts
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		log.Fatalf("MySQL ping failed: %v", err)
	}
	log.Println("Connected to MySQL")

	// Create or recreate table
	if *dropTable {
		log.Println("Dropping existing voucher table...")
		db.Exec("DROP TABLE IF EXISTS voucher")
	}

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS voucher (
			id BIGINT PRIMARY KEY,
			voucher_code VARCHAR(20) NOT NULL,
			is_valid BOOLEAN NOT NULL DEFAULT TRUE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`
	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	log.Println("Table 'voucher' ready")

	// Check existing records
	var existingCount int64
	db.QueryRow("SELECT COUNT(*) FROM voucher").Scan(&existingCount)
	if existingCount > 0 {
		log.Printf("Table already has %d records. Use --drop to recreate.", existingCount)
		if existingCount >= *totalRecords {
			log.Println("Already have enough records. Exiting.")
			return
		}
		log.Printf("Inserting remaining %d records starting from ID %d...",
			*totalRecords-existingCount, existingCount+1)
	}

	// Seed vouchers in batches
	startID := existingCount + 1
	totalToInsert := *totalRecords - existingCount
	inserted := int64(0)
	startTime := time.Now()
	lastLogTime := startTime

	for startID <= *totalRecords {
		endID := min(startID+int64(*batchSize)-1, *totalRecords)
		count := int(endID - startID + 1)

		// Build batch INSERT
		var sb strings.Builder
		sb.WriteString("INSERT INTO voucher (id, voucher_code, is_valid) VALUES ")

		for i := range count {
			id := startID + int64(i)
			isValid := rand.Intn(100) < *validPercentage
			voucherCode := generateVoucherCode(uint32(id), isValid)

			if i > 0 {
				sb.WriteString(",")
			}
			validInt := 0
			if isValid {
				validInt = 1
			}
			fmt.Fprintf(&sb, "(%d,'%s',%d)", id, voucherCode, validInt)
		}

		// Execute batch insert
		if _, err := db.Exec(sb.String()); err != nil {
			log.Printf("Failed to insert batch %d-%d: %v", startID, endID, err)
			// Retry logic: try one by one
			for i := range count {
				id := startID + int64(i)
				isValid := rand.Intn(100) < *validPercentage
				voucherCode := generateVoucherCode(uint32(id), isValid)
				db.Exec("INSERT IGNORE INTO voucher (id, voucher_code, is_valid) VALUES (?, ?, ?)",
					id, voucherCode, isValid)
			}
		}

		inserted += int64(count)
		startID = endID + 1

		// Log progress every 5 seconds
		now := time.Now()
		if now.Sub(lastLogTime) >= 5*time.Second || inserted >= totalToInsert {
			elapsed := now.Sub(startTime).Seconds()
			rate := float64(inserted) / elapsed
			pct := float64(inserted) / float64(totalToInsert) * 100
			eta := time.Duration(float64(totalToInsert-inserted)/rate) * time.Second
			log.Printf("Progress: %s / %s (%.1f%%) — %.0f records/s — ETA: %v",
				formatNum(inserted), formatNum(totalToInsert), pct, rate, eta.Round(time.Second))
			lastLogTime = now
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("Seeding complete: %s records in %v (%.0f records/s)",
		formatNum(inserted), elapsed.Round(time.Second), float64(inserted)/elapsed.Seconds())

	// Verify
	var finalCount int64
	db.QueryRow("SELECT COUNT(*) FROM voucher").Scan(&finalCount)
	var validCount int64
	db.QueryRow("SELECT COUNT(*) FROM voucher WHERE is_valid = TRUE").Scan(&validCount)
	log.Printf("Verification: total=%s, valid=%s (%.1f%%)",
		formatNum(finalCount), formatNum(validCount), float64(validCount)/float64(finalCount)*100)
}

// generateVoucherCode creates a voucher code using the snowflake format
func generateVoucherCode(voucherID uint32, isValid bool) string {
	var snowflakeID uint64
	if isValid {
		snowflakeID |= uint64(1) << ValidBitShift
	}
	snowflakeID |= (uint64(voucherID) & VoucherIDMask) << VoucherIDBitShift
	snowflakeID |= uint64(rand.Uint32()) & PINMask

	return encodeBase36(snowflakeID)
}

// encodeBase36 converts uint64 to uppercase base36 string
func encodeBase36(n uint64) string {
	if n == 0 {
		return "0"
	}
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var buf [13]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = chars[n%36]
		n /= 36
	}
	return string(buf[i:])
}

// formatNum formats a number with K/M suffix
func formatNum(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
