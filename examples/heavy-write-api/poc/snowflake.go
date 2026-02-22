package main

import (
	"fmt"
)

// Snowflake ID structure for voucher codes:
// Bit 63: Valid flag (1 = valid voucher, 0 = invalid)
// Bits 31-62: Voucher ID (32 bits, maps to voucher table primary key)
// Bits 0-30: PIN code (31 bits, random number)

const (
	ValidBitShift     = 63
	VoucherIDBitShift = 31
	PINMask           = 0x7FFFFFFF
	VoucherIDMask     = 0xFFFFFFFF
)

// GenerateVoucherBytes creates pre-encoded voucher bytes for high-performance distribution.
// Format: [code_bytes][null_separator][is_valid_byte]
// Returns a slice ready for lock-free consumption without unmarshaling.
func GenerateVoucherBytes(voucherID uint32, isValid bool, pin uint32) []byte {
	var snowflakeID uint64
	if isValid {
		snowflakeID |= uint64(1) << ValidBitShift
	}
	snowflakeID |= (uint64(voucherID) & VoucherIDMask) << VoucherIDBitShift
	snowflakeID |= uint64(pin) & PINMask

	code := EncodeBase36(snowflakeID)

	// Pack as: code + null + isValid byte (optimized for direct response)
	result := make([]byte, len(code)+2)
	copy(result, code)
	result[len(code)] = 0 // null separator
	if isValid {
		result[len(code)+1] = 1
	} else {
		result[len(code)+1] = 0
	}
	return result
}

// CastDBVoucherBytes casts DBVoucher to voucher bytes for high-performance distribution.
// Format: [code_bytes][null_separator][is_valid_byte]
// Returns a slice ready for lock-free consumption without unmarshaling.
func CastDBVoucherBytes(dbVoucher *DBVoucher) []byte {
	var snowflakeID uint64
	if dbVoucher.IsValid {
		snowflakeID |= uint64(1) << ValidBitShift
	}
	snowflakeID |= (uint64(dbVoucher.ID) & VoucherIDMask) << VoucherIDBitShift
	snowflakeID |= uint64(dbVoucher.ID) & PINMask

	code := EncodeBase36(snowflakeID)

	// Pack as: code + null + isValid byte (optimized for direct response)
	result := make([]byte, len(code)+2)
	copy(result, code)
	result[len(code)] = 0 // null separator
	if dbVoucher.IsValid {
		result[len(code)+1] = 1
	} else {
		result[len(code)+1] = 0
	}
	return result
}

// ParseVoucherBytes extracts code and isValid from pre-encoded bytes.
// Lock-free operation - no mutex, no allocation on hot path.
func ParseVoucherBytes(data []byte) (code string, isValid bool) {
	// Find null separator
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0 {
			code = string(data[:i])
			isValid = data[i+1] == 1
			return
		}
	}
	// Fallback: treat as code only
	return string(data), false
}

// ValidateVoucherCode performs stateless validation by decoding the snowflake ID.
// No database query needed - voucher validity is encoded in the ID itself.
func ValidateVoucherCode(code string) (isValid bool, voucherID, pin uint32, err error) {
	snowflakeID, err := DecodeBase36(code)
	if err != nil {
		return false, 0, 0, err
	}

	isValid = (snowflakeID >> ValidBitShift) == 1
	voucherID = uint32((snowflakeID >> VoucherIDBitShift) & VoucherIDMask)
	pin = uint32(snowflakeID & PINMask)

	return isValid, voucherID, pin, nil
}

// ValidateVoucherForPod checks if a voucher ID belongs to a specific pod's range.
func ValidateVoucherForPod(voucherID uint32, podIndex, totalPods int, totalVouchers int64) bool {
	vouchersPerPod := totalVouchers / int64(totalPods)
	start := int64(podIndex)*vouchersPerPod + 1
	end := int64(podIndex+1) * vouchersPerPod
	return int64(voucherID) >= start && int64(voucherID) <= end
}

// EncodeBase36 converts uint64 to uppercase base36 string
func EncodeBase36(n uint64) string {
	if n == 0 {
		return "0"
	}
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var buf [13]byte // Max 13 chars for uint64 in base36
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = chars[n%36]
		n /= 36
	}
	return string(buf[i:])
}

// DecodeBase36 converts uppercase base36 string to uint64
func DecodeBase36(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	var result uint64
	for _, c := range s {
		result *= 36
		if c >= '0' && c <= '9' {
			result += uint64(c - '0')
		} else if c >= 'A' && c <= 'Z' {
			result += uint64(c - 'A' + 10)
		} else if c >= 'a' && c <= 'z' {
			result += uint64(c - 'a' + 10)
		} else {
			return 0, fmt.Errorf("invalid character: %c", c)
		}
	}
	return result, nil
}
