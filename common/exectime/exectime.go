// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package exectime

import (
	"fmt"
	"io"
	"os"
	"time"
)

type Timer struct {
	out      io.Writer
	timeMap  map[string]time.Time
	statsMap map[string]*RunningStatistics
	debug    bool
}

var timer *Timer

func StatTimeCurrentBlock(name string) func() { return timer.StatTimeCurrentBlock(name) }
func (t *Timer) StatTimeCurrentBlock(key string) func() {
	if t.debug {
		start := time.Now()
		return func() {
			dur := time.Since(start)
			stat := t.statsMap[key]
			if stat == nil {
				t.statsMap[key] = NewRunningStatistics()
				t.statsMap[key].Push(dur)
			} else {
				t.statsMap[key].Push(dur)
			}
		}
	}
	return func() {}
}

func PrintStats() { timer.PrintStats() }
func (t *Timer) PrintStats() {
	if t.debug {
		separator := "\n\n\n******************************************Stats******************************************\n\n\n"
		_, err := t.out.Write([]byte(separator))
		if err != nil {
			fmt.Printf("Timer::PrintStats: error writing [%s]\n", err)
		}
		for key, stat := range t.statsMap {
			total := stat.Mean() * time.Duration(stat.N)
			msg := fmt.Sprintf("%s: avg=%s, std=%s, total=%s, ops/sec=%f\n", key, stat.Mean(), stat.StandardDeviation(), total, (1.0 / float64(stat.Mean().Seconds())))
			_, err = t.out.Write([]byte(msg))
			if err != nil {
				fmt.Printf("Timer::PrintStats: error writing [%s]\n", err)
			}
		}
	}
}

func TimeCurrentBlock(name string) func() { return timer.TimeCurrentBlock(name) }
func (t *Timer) TimeCurrentBlock(name string) func() {
	if t.debug {
		start := time.Now()
		return func() {
			msg := fmt.Sprintf("%s took %v\n", name, time.Since(start))
			_, err := t.out.Write([]byte(msg))
			if err != nil {
				fmt.Printf("Timer::TimeCurrentBlock for %s: error [%s]\n", name, err)
			}
		}
	} else {
		return func() {}
	}
}

func SwitchOnDebug() { timer.SwitchOnDebug() }
func (t *Timer) SwitchOnDebug() {
	t.debug = true
}

func SwitchOffDebug() { timer.SwitchOffDebug() }
func (t *Timer) SwitchOffDebug() {
	t.debug = false
}

func Start(key string) { timer.Start(key) }
func (t *Timer) Start(key string) {
	t.timeMap[key] = time.Now()
}

func Stop(key string) { timer.Stop(key) }
func (t *Timer) Stop(key string) {
	last := t.timeMap[key]
	msg := fmt.Sprintf("%s took %v\n", key, time.Since(last))
	_, err := t.out.Write([]byte(msg))
	if err != nil {
		fmt.Printf("Timer::TimeCurrentBlock for %s: error [%s]", key, err)
	}
}

func New(out io.Writer, debug bool) *Timer {
	return &Timer{
		out:      out,
		debug:    debug,
		timeMap:  make(map[string]time.Time),
		statsMap: make(map[string]*RunningStatistics),
	}
}

func SetDefault(out io.Writer, debug bool) {
	timer = &Timer{
		out:      out,
		debug:    debug,
		timeMap:  make(map[string]time.Time),
		statsMap: make(map[string]*RunningStatistics),
	}
}

func init() {
	timer = New(os.Stdout, true)
}
