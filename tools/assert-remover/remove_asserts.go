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

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

func assert(cond bool, msg ...interface{}) {
	if !cond {
		if len(msg) != 0 {
			log.Panicf("Assertion failed: %v", msg)
		} else {
			log.Panicf("Assertion failed!")
		}
	}
}

func removeFunctionCalls(code, funcName string) string {
	var result strings.Builder
	pattern := fmt.Sprintf(`^[ \t]*%s[ \t]*\(`, regexp.QuoteMeta(funcName))
	re := regexp.MustCompile(pattern)
	i := 0

	for i < len(code) {
		loc := re.FindStringIndex(code[i:])
		if loc == nil {
			// No match, copy input to output.
			result.WriteByte(code[i])
			i++
			continue
		}

		matchLen := loc[1] - loc[0]
		// Must be at least the length of funcName.
		assert(matchLen > len(funcName), matchLen, funcName)

		// Skip matched bytes, except the starting parenthesis.
		i += (matchLen - 1)
		assert(i < len(code), i, len(code))
		assert(code[i] == '(', i, code[i])

		depth := 0
		j := i
		for ; j < len(code); j++ {
			if code[j] == '(' {
				depth++
			} else if code[j] == ')' {
				depth--
				if depth == 0 {
					// Gobble till end of line, in case there is some comment after the assert.
					for ; j < len(code); j++ {
						if code[j] == '\n' {
							break
						}
					}
					break
				}
			} else if code[j] == '\n' {
				// Keep line number same for easier error lookup.
				result.WriteByte(code[j])
			}
		}

		// Parenthesis must match completely.
		assert(depth == 0, depth)

		i = j
	}

	return result.String()
}

func removeIfBlock(code, ifPreamble string) string {
	var result strings.Builder
	pattern := fmt.Sprintf(`^[ \t]*%s[ \t]*\{`, regexp.QuoteMeta(ifPreamble))
	re := regexp.MustCompile(pattern)
	i := 0

	for i < len(code) {
		loc := re.FindStringIndex(code[i:])
		if loc == nil {
			// No match, copy input to output.
			result.WriteByte(code[i])
			i++
			continue
		}

		matchLen := loc[1] - loc[0]
		// Must be at least the length of ifPreamble.
		assert(matchLen > len(ifPreamble), matchLen, ifPreamble)

		// Skip matched bytes, except the starting parenthesis.
		i += (matchLen - 1)
		assert(i < len(code), i, len(code))
		assert(code[i] == '{', i, code[i])

		depth := 0
		j := i
		for ; j < len(code); j++ {
			if code[j] == '{' {
				depth++
			} else if code[j] == '}' {
				depth--
				if depth == 0 {
					// Gobble till end of line, in case there is some comment after the assert.
					for ; j < len(code); j++ {
						if code[j] == '\n' {
							//result.WriteByte(code[j])
							break
						}
					}
					break
				}
			} else if code[j] == '\n' {
				// Keep line number same for easier error lookup.
				result.WriteByte(code[j])
			}
		}

		// Parenthesis must match completely.
		assert(depth == 0, depth)

		i = j
	}

	return result.String()
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <go file to remove asserts from>\n", os.Args[0])
		os.Exit(1)
	}
	fileToFix := os.Args[1]
	saveFile := fileToFix + ".orig.withasserts"

	// Read input file.
	data, err := ioutil.ReadFile(fileToFix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", fileToFix, err)
		os.Exit(1)
	}

	err = ioutil.WriteFile(saveFile, data, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file %s: %v\n", saveFile, err)
		os.Exit(1)
	}

	cleaned := string(data)

	//
	// Remove conditional debug build blocks.
	//
	ifPreamblesToRemove := []string{
		"if common.IsDebugBuild()",
	}

	for _, ifPreambleToRemove := range ifPreamblesToRemove {
		// Remove if preamble.
		cleaned = removeIfBlock(cleaned, ifPreambleToRemove)
		if os.Getenv("DEBUG") == "1" {
			fmt.Printf("Removed all %s blocks from %s\n", ifPreambleToRemove, fileToFix)
		}
	}

	//
	// Remove Asserts.
	//
	funcsToRemove := []string{
		"common.Assert",
		"Assert",
		"log.Debug",
	}

	for _, funcToRemove := range funcsToRemove {
		// Remove function calls
		cleaned = removeFunctionCalls(cleaned, funcToRemove)
		if os.Getenv("DEBUG") == "1" {
			fmt.Printf("Removed all calls to %s from %s\n", funcToRemove, fileToFix)
		}
	}

	// Write to output file
	err = ioutil.WriteFile(fileToFix, []byte(cleaned), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file %s: %v\n", fileToFix, err)
		os.Exit(1)
	}
}
