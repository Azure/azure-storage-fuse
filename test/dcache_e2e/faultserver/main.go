// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

// dcache-fault-server is a test-only distributed-cache protocol server used by
// the kind E2E suite. Each mode emits one deterministic failure shape.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	pb "github.com/nearora-msft/dist-cache-client-go/proto"
	"google.golang.org/protobuf/proto"
)

const checksumMetadataKey = "CHUNK_CHECKSUM"

var (
	mode          = flag.String("mode", "", "fault mode")
	listenAddr    = flag.String("listen", ":9065", "cache protocol listen address")
	azureListen   = flag.String("azure-listen", "", "optional Azure proxy listen address")
	azureUpstream = flag.String("azure-upstream", "", "Azure proxy upstream URL")

	downloadRequests atomic.Uint64
	uploadRequests   atomic.Uint64
	azureFailures    atomic.Uint64
)

func main() {
	flag.Parse()
	if *mode == "" {
		log.Fatal("-mode is required")
	}

	if *azureListen != "" {
		if *azureUpstream == "" {
			log.Fatal("-azure-upstream is required with -azure-listen")
		}
		go serveAzureProxy(*azureListen, *azureUpstream)
	}

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", *listenAddr, err)
	}
	log.Printf("event=ready mode=%s addr=%s", *mode, listener.Addr())
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("accept: %v", err)
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	for {
		req, err := readRequest(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("event=request_error error=%q", err)
			}
			return
		}

		switch payload := req.Payload.(type) {
		case *pb.Request_Downloadrequest:
			count := downloadRequests.Add(1)
			log.Printf("event=download mode=%s count=%d key=%q", *mode, count, payload.Downloadrequest.Filename)
			if !handleDownload(conn) {
				return
			}
		case *pb.Request_Uploadrequest:
			if _, err := io.CopyN(io.Discard, conn, int64(payload.Uploadrequest.Filesize)); err != nil {
				log.Printf("event=upload_read_error error=%q", err)
				return
			}
			count := uploadRequests.Add(1)
			log.Printf("event=upload count=%d key=%q", count, payload.Uploadrequest.Filename)
			if err := writeResponse(conn, &pb.UploadResponse{Result: pb.UploadResponse_SUCCESS}, nil); err != nil {
				return
			}
		default:
			log.Printf("event=unsupported_request")
			return
		}
	}
}

func readRequest(conn net.Conn) (*pb.Request, error) {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size > 64*1024*1024 {
		return nil, fmt.Errorf("request too large: %d", size)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	var req pb.Request
	if err := proto.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func handleDownload(conn net.Conn) bool {
	switch *mode {
	case "miss-got-lock":
		return writeDownload(conn, pb.DownloadResponse_NOT_FOUND_GOT_LOCK, 0, nil, nil)
	case "checksum-mismatch":
		data := []byte("corrupt-cache-data")
		metadata := map[string][]byte{
			checksumMetadataKey: []byte(strconv.FormatUint(uint64(crc32.ChecksumIEEE(data)^0xFFFFFFFF), 10)),
		}
		return writeDownload(conn, pb.DownloadResponse_SUCCESS, uint64(len(data)), metadata, data)
	case "zero-byte":
		metadata := map[string][]byte{checksumMetadataKey: []byte("0")}
		return writeDownload(conn, pb.DownloadResponse_SUCCESS, 0, metadata, nil)
	case "locked-timeout":
		return writeDownload(conn, pb.DownloadResponse_NOT_FOUND_ALREADY_LOCKED, 0, nil, nil)
	case "request-timeout":
		time.Sleep(35 * time.Second)
		return false
	case "malformed-protobuf":
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], 1)
		_, _ = conn.Write(header[:])
		_, _ = conn.Write([]byte{0xFF})
		return false
	case "truncated-payload":
		resp := &pb.DownloadResponse{
			Result:   pb.DownloadResponse_SUCCESS,
			Filesize: 16 * 1024 * 1024,
			Metadata: map[string][]byte{checksumMetadataKey: []byte("0")},
		}
		body, err := proto.Marshal(resp)
		if err != nil {
			return false
		}
		if err := writeFrame(conn, body); err != nil {
			return false
		}
		_, _ = conn.Write([]byte("short"))
		return false
	default:
		log.Printf("event=invalid_mode mode=%s", *mode)
		return false
	}
}

func writeDownload(conn net.Conn, result pb.DownloadResponse_Result, size uint64, metadata map[string][]byte, data []byte) bool {
	return writeResponse(conn, &pb.DownloadResponse{
		Result:   result,
		Filesize: size,
		Metadata: metadata,
	}, data) == nil
}

func writeResponse(conn net.Conn, response proto.Message, data []byte) error {
	body, err := proto.Marshal(response)
	if err != nil {
		return err
	}
	if err := writeFrame(conn, body); err != nil {
		return err
	}
	if len(data) > 0 {
		_, err = conn.Write(data)
	}
	return err
}

func writeFrame(conn net.Conn, body []byte) error {
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(body)))
	if _, err := conn.Write(header[:]); err != nil {
		return err
	}
	_, err := conn.Write(body)
	return err
}

func serveAzureProxy(addr, upstream string) {
	target, err := url.Parse(upstream)
	if err != nil {
		log.Fatalf("parse Azure upstream: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("event=azure_proxy_error error=%q", err)
		http.Error(w, "proxy error", http.StatusBadGateway)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.Header.Get("x-ms-range") != "" {
			count := azureFailures.Add(1)
			log.Printf("event=azure_get_failed count=%d path=%q", count, req.URL.Path)
			w.Header().Set("x-ms-error-code", "AuthenticationFailed")
			http.Error(w, "injected Azure GET failure", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, req)
	})

	log.Printf("event=azure_proxy_ready addr=%s upstream=%s", addr, target.Redacted())
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Printf("event=azure_proxy_stopped error=%q", err)
		os.Exit(1)
	}
}
