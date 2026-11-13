// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package exectime

import (
	"math"
	"time"
)

type RunningStatistics struct {
	N    uint64
	oldM time.Duration
	newM time.Duration
	oldS time.Duration
	newS time.Duration
}

func NewRunningStatistics() *RunningStatistics {
	return &RunningStatistics{
		N: 0,
	}
}

func (rs *RunningStatistics) Push(dur time.Duration) {
	rs.N++
	if rs.N == 1 {
		rs.oldM = dur
		rs.newM = dur
		rs.oldS = 0
		return
	}

	rs.newM = rs.oldM + ((dur - rs.oldM) / time.Duration(rs.N))
	rs.newS = rs.oldS + (dur-rs.oldM)*(dur-rs.newM)

	rs.oldM = rs.newM
	rs.oldS = rs.newS
}

func (rs *RunningStatistics) Mean() time.Duration {
	return rs.newM
}

func (rs *RunningStatistics) Variance() time.Duration {
	if rs.N > 1 {
		return rs.newS / time.Duration(rs.N-1)
	}
	return time.Duration(0)
}

func (rs *RunningStatistics) StandardDeviation() time.Duration {
	dev := math.Sqrt(float64(rs.Variance()))
	return time.Duration(dev)
}
