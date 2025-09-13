/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package common

import (
	"fmt"
	"math/rand"
	"time"
)

//go:generate $ASSERT_REMOVER $GOFILE

type debugLogger func(format string, args ...interface{})

var logD debugLogger

type ProbabilityValue float32

// Discrete probability values to be used for InjectError().
const (
	PROB_NEVER ProbabilityValue = 0

	// 1 in million.
	PROB_VERY_LOW ProbabilityValue = 1.0e-6

	// 1 in 10,000.
	PROB_LOW ProbabilityValue = 1.0e-4

	// 1 in 1000.
	PROB_MODERATE ProbabilityValue = 1.0e-3

	// 1 in 100.
	PROB_HIGH ProbabilityValue = 1.0e-2

	// 1 in 10.
	PROB_VERY_HIGH ProbabilityValue = 1.0e-1

	PROB_ALWAYS ProbabilityValue = 1
)

//
// InjectError() can be used to inject an error with the given probability.
// In non-debug builds it's a no-op.
//
// Sample usage:
//
// err := InjectError(PROB_VERY_LOW, "Simulating component RV mismatch for", mvName)
// if err != nil {
//     return err
// }
//
// err := InjectError(PROB_MODERATE)
// if err != nil {
//     return err
// }
//

func InjectError(prob ProbabilityValue, msg ...interface{}) error {
	if !IsDebugBuild() {
		return nil
	}

	inject := rand.Float64() < float64(prob)
	if !inject {
		return nil
	}

	if len(msg) != 0 {
		return fmt.Errorf("InjectError:: [probability=%v]: %v", prob, msg)
	} else {
		return fmt.Errorf("InjectError:: [probability=%v]", prob)
	}
}

//
// InjectSleep() can be used to inject a random sleep, where duration is randomly selected from the
// given range. The sleep is not added always, but with a probability specified by the first argument.
// In non-debug builds it's a no-op.
//
// Sample usage:
//
// Following will randomly inject a sleep between 0 to 5 secs.
// InjectSleep(PROB_MODERATE, 0, 5*Time.Second, "simulating delay in get blob")
//
// Following will always inject a fixed sleep of 100ms.
// InjectSleep(PROB_ALWAYS, 100*Time.Millisecond, 100*Time.Millisecond)
//

func InjectSleep(prob ProbabilityValue, minDuration, maxDuration time.Duration, msg ...interface{}) {
	if !IsDebugBuild() || logD == nil {
		return
	}

	Assert(maxDuration >= minDuration, minDuration, maxDuration)

	inject := rand.Float64() < float64(prob)
	if !inject {
		return
	}

	randNsecs := rand.Int63n(maxDuration.Nanoseconds() - minDuration.Nanoseconds())
	sleepDuration := time.Duration(minDuration.Nanoseconds() + randNsecs)

	if len(msg) != 0 {
		logD("InjectSleep:: [%v, %v] Sleeping for %v: %v", sleepDuration, msg)
	} else {
		logD("InjectSleep:: [%v, %v] Sleeping for %v", sleepDuration)
	}

	time.Sleep(sleepDuration)
}

// This must be called to set the logger, before InjectSleep() can be used.
func InitErrorInjection(logger debugLogger) {
	logD = logger
}
