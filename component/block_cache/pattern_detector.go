package block_cache

import (
	"math"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// PatternDetector is used to detect read patterns in file access.

type patternType int32

const (
	patternUnknown    patternType = 0
	patternSequential patternType = 1
	patternRandom     patternType = 2
)

// Map patternType values to their string representations
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

type patternDetector struct {
	prevOffset atomic.Int64

	// This can toggle between sequential and random access.
	// allowedRange: (..-3, 3..)
	// we use 3 streak to confirm the access pattern.
	// <= -3: strongly random access
	// >=  3: strongly sequential access
	streak atomic.Int32

	nxtReadAheadBlockIdx atomic.Int64

	// Debug info
	fileName string
}

func newPatternDetector() *patternDetector {
	pd := &patternDetector{
		prevOffset: atomic.Int64{},
		streak:     atomic.Int32{},
	}

	//We start with sequential access pattern
	pd.streak.Store(int32(patternSequential))

	return pd
}

// updateAccessPattern updates the access pattern based on the current offset.
func (pd *patternDetector) updateAccessPattern(currentOffset int64) patternType {
	prevOffset := pd.prevOffset.Swap(currentOffset)
	// TODO: Use more sophisticated logic to detect access patterns.
	windowSize := int64(bc.blockSize) * 2 // 2 blocks window

	absDiff := int64(math.Abs(float64(currentOffset - prevOffset)))

	if absDiff <= windowSize {
		// Sequential access

		if pd.streak.Add(1) > 0 {
			return patternSequential
		} else {
			// Reset streak
			log.Debug("PatternDetector::updateAccessPattern: Access pattern changed from Random to Sequential for file %s",
				pd.fileName)
			pd.streak.Store(0)

			return patternUnknown
		}
	} else {
		// Random access

		if pd.streak.Add(-1) < 0 {
			return patternRandom
		} else {
			// Reset streak
			log.Debug("PatternDetector::updateAccessPattern: Access pattern changed from Sequential to Random for file %s",
				pd.fileName)
			pd.streak.Store(0)
			return patternUnknown
		}
	}
}
