package block_cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPatternDetector(t *testing.T) {
	pd := newPatternDetector()
	
	assert.NotNil(t, pd)
	assert.Equal(t, int32(3), pd.streak.Load(), "Initial streak should be 3 for sequential access")
	assert.Equal(t, int64(0), pd.prevOffset.Load(), "Initial prevOffset should be 0")
}

func TestUpdateAccessPattern_Sequential(t *testing.T) {
	// Setup: Mock bc with blockSize
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}
	
	pd := newPatternDetector()
	
	// Test sequential access pattern
	// Start at offset 0
	pattern := pd.updateAccessPattern(0)
	assert.Equal(t, patternSequential, pattern, "Should detect sequential pattern initially")
	
	// Continue sequentially within 2 block window
	pattern = pd.updateAccessPattern(1024 * 1024) // 1 MB offset
	assert.Equal(t, patternSequential, pattern, "Should continue sequential pattern")
	
	pattern = pd.updateAccessPattern(2 * 1024 * 1024) // 2 MB offset
	assert.Equal(t, patternSequential, pattern, "Should maintain sequential pattern")
}

func TestUpdateAccessPattern_Random(t *testing.T) {
	// Setup: Mock bc with blockSize
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}
	
	pd := newPatternDetector()
	
	// Reset streak to 0 for clean test
	pd.streak.Store(0)
	
	// Test random access pattern - jumps beyond 2 block window
	pd.updateAccessPattern(0)
	pd.updateAccessPattern(10 * 1024 * 1024) // Jump to 10 MB
	pd.updateAccessPattern(50 * 1024 * 1024) // Jump to 50 MB
	pattern := pd.updateAccessPattern(100 * 1024 * 1024) // Jump to 100 MB
	
	assert.Equal(t, patternRandom, pattern, "Should detect random pattern after multiple jumps")
}

func TestUpdateAccessPattern_TransitionFromSequentialToRandom(t *testing.T) {
	// Setup: Mock bc with blockSize
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}
	
	pd := newPatternDetector()
	
	// Start with sequential
	pd.updateAccessPattern(0)
	pd.updateAccessPattern(1024 * 1024)
	pd.updateAccessPattern(2 * 1024 * 1024)
	
	// Now jump randomly multiple times
	pd.updateAccessPattern(100 * 1024 * 1024)
	pd.updateAccessPattern(200 * 1024 * 1024)
	pd.updateAccessPattern(300 * 1024 * 1024)
	pattern := pd.updateAccessPattern(400 * 1024 * 1024)
	
	assert.Equal(t, patternRandom, pattern, "Should transition to random pattern")
}

func TestUpdateAccessPattern_TransitionFromRandomToSequential(t *testing.T) {
	// Setup: Mock bc with blockSize
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}
	
	pd := newPatternDetector()
	
	// Start with random pattern
	pd.streak.Store(-3) // Set to random
	pd.prevOffset.Store(100 * 1024 * 1024)
	
	// Now read sequentially
	pd.updateAccessPattern(100 * 1024 * 1024)
	pd.updateAccessPattern(101 * 1024 * 1024)
	pd.updateAccessPattern(102 * 1024 * 1024)
	pattern := pd.updateAccessPattern(103 * 1024 * 1024)
	
	assert.Equal(t, patternSequential, pattern, "Should transition to sequential pattern")
}

func TestUpdateAccessPattern_WindowSize(t *testing.T) {
	// Setup: Mock bc with blockSize
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}
	
	pd := newPatternDetector()
	pd.streak.Store(0)
	
	// Test boundary: exactly at window size (2 blocks = 2 MB)
	pd.updateAccessPattern(0)
	pattern := pd.updateAccessPattern(2 * 1024 * 1024)
	assert.NotEqual(t, patternRandom, pattern, "2MB offset should be within window")
	
	// Test just beyond window size
	pd.streak.Store(0)
	pd.updateAccessPattern(0)
	pattern = pd.updateAccessPattern(2*1024*1024 + 1)
	assert.Equal(t, patternUnknown, pattern, "Just beyond window should start transitioning")
}

func TestPatternType_String(t *testing.T) {
	assert.Equal(t, "patternUnknown", patternUnknown.String())
	assert.Equal(t, "patternSequential", patternSequential.String())
	assert.Equal(t, "patternRandom", patternRandom.String())
	assert.Equal(t, "Unknown", patternType(99).String())
}

func TestUpdateAccessPattern_StreakBoundaries(t *testing.T) {
	// Setup: Mock bc with blockSize
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}
	
	pd := newPatternDetector()
	
	// Test that streak correctly increments and returns pattern at threshold
	pd.streak.Store(2)
	pattern := pd.updateAccessPattern(0)
	pd.updateAccessPattern(1024 * 1024)
	assert.Equal(t, patternSequential, pattern, "Should return sequential at streak >= 3")
	
	// Test negative streak
	pd.streak.Store(-2)
	pd.updateAccessPattern(0)
	pd.updateAccessPattern(100 * 1024 * 1024)
	pd.updateAccessPattern(200 * 1024 * 1024)
	pattern = pd.updateAccessPattern(300 * 1024 * 1024)
	assert.Equal(t, patternRandom, pattern, "Should return random at streak <= -3")
}
