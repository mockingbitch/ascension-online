// Package fixedpoint holds integer-math helpers for game quantities.
// Never use float64 for currency, cultivation or resources (see .claude/rules/data-structures.md).
package fixedpoint

import "math"

// BPScale is 100% expressed in basis points.
const BPScale = 10_000

// MilliScale is the scale of *_milli rate fields.
const MilliScale = 1_000

// MulBP returns v * bp / 10000, saturating at math.MaxInt64.
func MulBP(v int64, bp int64) int64 {
	return MulDiv(v, bp, BPScale)
}

// MulDiv returns a*b/c with saturation instead of overflow. a, b >= 0, c > 0.
func MulDiv(a, b, c int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > math.MaxInt64/b {
		// Fall back to dividing first; loses precision only for huge values.
		q := a / c
		if q > math.MaxInt64/b {
			return math.MaxInt64
		}
		return q*b + (a%c)*b/c
	}
	return a * b / c
}

// SatAdd adds two non-negative int64 values, saturating at math.MaxInt64.
func SatAdd(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

// Clamp bounds v to [lo, hi].
func Clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
