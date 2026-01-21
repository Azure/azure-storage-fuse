package block_cache

import (
	"math"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// patternDetector analyzes file access patterns to optimize read-ahead behavior.
//
// Overview:
//
// PatternDetector tracks read operations to detect whether a file is being
// accessed sequentially or randomly. This information is used to:
//
//   - Enable read-ahead for sequential access (improves throughput)
//   - Disable read-ahead for random access (avoids cache pollution)
//
// Detection Algorithm:
//
// The detector uses a "streak" counter that moves between -3 (strongly random)
// and +3 (strongly sequential):
//
//   - Sequential access increments streak (moves toward +3)
//   - Random access decrements streak (moves toward -3)
//   - |streak| >= 3 indicates confident pattern detection
//   - |streak| < 3 indicates transition or uncertain pattern
//
// Sequential vs Random:
//
// Access is considered sequential if the difference between consecutive offsets
// is within a 2-block window (2 * blockSize). This tolerance accounts for:
//
//   - Slight reordering of requests
//   - Small backward seeks (e.g., re-reading headers)
//   - Block-aligned operations
//
// Access outside this window is considered random.
//
// Thread Safety:
//
// All fields use atomic operations, allowing thread-safe pattern detection
// without locks. This is important because pattern detection happens on the
// hot path of read operations.
//
// Why Per-Handle Detection:
//
// Each handle has its own pattern detector because different handles to the
// same file may have different access patterns. For example:
//   - Thread 1: Sequential scan (grep)
//   - Thread 2: Random access (database queries)
//
// Per-handle detection optimizes for each pattern independently.

// patternType represents the detected access pattern.
type patternType int32

const (
	// patternUnknown indicates the pattern is not yet clear or is transitioning.
	// No read-ahead is performed in this state.
	patternUnknown patternType = 0

	// patternSequential indicates sequential access with high confidence.
	// Read-ahead is enabled to prefetch upcoming blocks.
	patternSequential patternType = 1

	// patternRandom indicates random access with high confidence.
	// Read-ahead is disabled to avoid polluting the cache.
	patternRandom patternType = 2
)

// String returns a human-readable representation of the pattern type.
// Useful for logging and debugging.
func (pt patternType) String() string {
	switch pt {
	case patternUnknown:
		return "patternUnknown"
	case patternSequential:
		return "patternSequential"
	case patternRandom:
		return "patternRandom"
	default:
		return "Unknown"
	}
}

// patternDetector tracks read access patterns for a single handle.
//
// Fields:
//   - prevOffset: Last read offset (for calculating access delta)
//   - streak: Confidence counter (-3 to +3, indicates pattern strength)
//   - nxtReadAheadBlockIdx: Next block index to prefetch (for sequential access)
//   - fileName: File name (for debugging/logging)
//
// Algorithm Details:
//
// The streak counter provides hysteresis to avoid thrashing between patterns:
//   - Requires 3 consecutive sequential accesses to enable read-ahead
//   - Requires 3 consecutive random accesses to disable read-ahead
//   - Intermediate values indicate transition or mixed patterns
//
// This prevents toggling read-ahead on/off for mixed access patterns,
// which would waste cache space and bandwidth.
type patternDetector struct {
	prevOffset atomic.Int64 // Previous read offset (for delta calculation)

	// Streak counter tracking pattern confidence.
	// Range: -3 (strongly random) to +3 (strongly sequential)
	//
	// Interpretation:
	//   streak >= +3: Sequential pattern confirmed, enable read-ahead
	//   -3 < streak < +3: Uncertain or transitioning, disable read-ahead
	//   streak <= -3: Random pattern confirmed, disable read-ahead
	//
	// The 3-count requirement provides hysteresis to avoid pattern oscillation.
	streak atomic.Int32

	// Next block index to prefetch for sequential reads.
	// Only used when pattern is sequential. Tracks which blocks have been
	// prefetched to avoid duplicate prefetch requests.
	nxtReadAheadBlockIdx atomic.Int64

	// Debug info - file name for logging
	fileName string
}

// newPatternDetector creates a new pattern detector with default settings.
//
// Returns a pattern detector initialized to assume sequential access.
// This optimistic initialization enables read-ahead immediately for
// truly sequential workloads.
//
// Starting with streak=3 means:
//   - First sequential access keeps read-ahead enabled
//   - First random access reduces to streak=2 (still sequential)
//   - Needs 3 random accesses to disable read-ahead
//
// This is appropriate because:
//   - Many workloads are primarily sequential
//   - False-positive read-ahead is cheaper than missed opportunities
//   - Random workloads quickly adjust the pattern
func newPatternDetector() *patternDetector {
	pd := &patternDetector{
		prevOffset: atomic.Int64{},
		streak:     atomic.Int32{},
	}

	//We start with sequential access pattern
	pd.streak.Store(int32(3))

	return pd
}

// updateAccessPattern analyzes a read operation and updates the access pattern.
//
// This method is called before each read to:
//  1. Compare current offset with previous offset
//  2. Determine if access is sequential or random
//  3. Update streak counter accordingly
//  4. Return the detected pattern
//
// Parameters:
//   - currentOffset: File offset of the current read operation
//
// Returns the detected pattern type (sequential, random, or unknown).
//
// Algorithm:
//
//  1. Calculate delta = abs(currentOffset - prevOffset)
//  2. If delta <= windowSize (2 blocks): sequential, increment streak
//  3. If delta > windowSize: random, decrement streak
//  4. If |streak| >= 3: return confident pattern
//  5. If crossing zero: reset streak (pattern transition)
//
// Window Size:
//
// The window size (2 * blockSize) provides tolerance for:
//   - Minor backward seeks (e.g., re-reading headers)
//   - Block-aligned I/O that skips small amounts
//   - OS-level read-ahead that reorders slightly
//
// This prevents classifying slightly-non-sequential patterns as random.
//
// Streak Reset:
//
// When the streak crosses zero (e.g., from +1 to -1), it's reset to 0.
// This prevents accumulated history from delaying pattern transitions.
// For example, after 100 sequential reads, a single random read shouldn't
// require 103 more random reads to change the pattern.
//
// Thread Safety:
//
// Uses atomic operations throughout, allowing concurrent calls without locks.
// The pattern may be slightly inaccurate under high concurrency, but this is
// acceptable for a heuristic optimization.
func (pd *patternDetector) updateAccessPattern(currentOffset int64) patternType {
	prevOffset := pd.prevOffset.Swap(currentOffset)
	windowSize := int64(bc.blockSize) * 2 // 2 blocks window

	absDiff := int64(math.Abs(float64(currentOffset - prevOffset)))

	if absDiff <= windowSize {
		// Sequential access
		newStreak := pd.streak.Add(1)
		if newStreak >= 3 {
			return patternSequential
		} else if newStreak < 0 {
			// Reset streak
			log.Debug("PatternDetector::updateAccessPattern: Access pattern changed from Random to Sequential for file %s",
				pd.fileName)
			pd.streak.Store(0)
		}
	} else {
		// Random access
		newStreak := pd.streak.Add(-1)
		if newStreak <= -3 {
			return patternRandom
		} else if newStreak > 0 {
			// Reset streak
			log.Debug("PatternDetector::updateAccessPattern: Access pattern changed from Sequential to Random for file %s",
				pd.fileName)
			pd.streak.Store(0)
		}
	}

	return patternUnknown
}
