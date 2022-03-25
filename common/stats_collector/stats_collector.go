/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package stats

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var Blobfuse2Stats *Stats

// http://localhost:1234//getStats/fuse?format=YaML
func GetFuseStats(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetFuseStats")
	out_format := r.URL.Query().Get("format")
	if out_format != "" {
		out_format = strings.ToLower(out_format)
	}

	if out_format == "yaml" || out_format == "yml" {
		d, err := yaml.Marshal(&Blobfuse2Stats.fuse)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Fprintf(w, string(d))
		return
	}
	json.NewEncoder(w).Encode(&Blobfuse2Stats.fuse)

}

func GetAttrCacheStats(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetAttrCacheStats")
	out_format := r.URL.Query().Get("format")
	if out_format != "" {
		out_format = strings.ToLower(out_format)
	}

	if out_format == "yaml" || out_format == "yml" {
		d, err := yaml.Marshal(&Blobfuse2Stats.attrCache)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Fprintf(w, string(d))
		return
	}
	json.NewEncoder(w).Encode(&Blobfuse2Stats.attrCache)

}

func GetFileCacheStats(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetFileCacheStats")
	out_format := r.URL.Query().Get("format")
	if out_format != "" {
		out_format = strings.ToLower(out_format)
	}

	if out_format == "yaml" || out_format == "yml" {
		d, err := yaml.Marshal(&Blobfuse2Stats.fileCache)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Fprintf(w, string(d))
		return
	}
	json.NewEncoder(w).Encode(&Blobfuse2Stats.fileCache)

}

func GetStorageStats(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetStorageStats")
	out_format := r.URL.Query().Get("format")
	if out_format != "" {
		out_format = strings.ToLower(out_format)
	}

	if out_format == "yaml" || out_format == "yml" {
		d, err := yaml.Marshal(&Blobfuse2Stats.storage)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Fprintf(w, string(d))
		return
	}
	json.NewEncoder(w).Encode(&Blobfuse2Stats.storage)
}

func GetCommonStats(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetCommonStats")
	out_format := r.URL.Query().Get("format")
	if out_format != "" {
		out_format = strings.ToLower(out_format)
	}

	if out_format == "yaml" || out_format == "yml" {
		d, err := yaml.Marshal(&Blobfuse2Stats.common)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Fprintf(w, string(d))
		return
	}
	json.NewEncoder(w).Encode(&Blobfuse2Stats.common)
}

func GetStats(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetStats")
	out_format := r.URL.Query().Get("format")
	if out_format != "" {
		out_format = strings.ToLower(out_format)
	}

	if out_format == "yaml" || out_format == "yml" {
		d, err := yaml.Marshal(&Blobfuse2Stats)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Fprintf(w, string(d))
		return
	}
	json.NewEncoder(w).Encode(&Blobfuse2Stats)
}

func allocate() *Stats {
	var stats *Stats

	stats = &Stats{}
	stats.fuse.lck = sync.RWMutex{}
	stats.attrCache.lck = sync.RWMutex{}
	stats.fileCache.lck = sync.RWMutex{}
	stats.storage.lck = sync.RWMutex{}
	stats.common.lck = sync.RWMutex{}

	return stats
}

// StartRESTServer : Main server method to start the server
func StartRESTServer(port int) {
	Blobfuse2Stats = allocate()

	http.HandleFunc("/getstats/fuse", GetFuseStats)
	http.HandleFunc("/getstats/attr", GetAttrCacheStats)
	http.HandleFunc("/getstats/file", GetFileCacheStats)
	http.HandleFunc("/getstats/storage", GetStorageStats)
	http.HandleFunc("/getstats/common", GetCommonStats)
	http.HandleFunc("/getstats", GetStats)

	portStr := fmt.Sprintf(":%d", port)
	log.Fatal(http.ListenAndServe(portStr, nil))
}
