// Copyright 2025 Azure Corporation. All rights reserved.
// Licensed under the MIT License.

package log

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OTelLogExporter manages the OpenTelemetry log exporter with gRPC connection
type OTelLogExporter struct {
	conn       *grpc.ClientConn
	exporter   log.Exporter
	processor  log.LogRecordProcessor
	provider   *log.LoggerProvider
	endpoint   string
	mu         sync.RWMutex
	closed     bool
}

// NewOTelLogExporter creates a new OTel log exporter with gRPC connection to collector
func NewOTelLogExporter(ctx context.Context, endpoint string, serviceName string) (*OTelLogExporter, error) {
	if endpoint == "" {
		endpoint = "localhost:4317" // Default OTel collector endpoint
	}

	// Create gRPC connection
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to otel collector: %w", err)
	}

	// Create OTLP log exporter
	exporter, err := otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(conn))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create OTLP log exporter: %w", err)
	}

	// Create resource for service identification
	res, err := resource.New(ctx,
		resource.WithAttributes(
			// Add service attributes here
		),
	)
	if err != nil {
		exporter.Shutdown(ctx)
		conn.Close()
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create batch processor with 1-minute interval
	processor := log.NewBatchProcessor(
		exporter,
		log.WithBatchTimeout(1*time.Minute),
		log.WithMaxQueueSize(10000),
		log.WithMaxExportBatchSize(512),
	)

	// Create logger provider
	provider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(processor),
	)

	otel.SetLoggerProvider(provider)

	return &OTelLogExporter{
		conn:      conn,
		exporter:  exporter,
		processor: processor,
		provider:  provider,
		endpoint:  endpoint,
	}, nil
}

// GetLoggerProvider returns the OpenTelemetry logger provider
func (o *OTelLogExporter) GetLoggerProvider() *log.LoggerProvider {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.provider
}

// Shutdown gracefully closes the exporter and underlying connections
func (o *OTelLogExporter) Shutdown(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return nil
	}

	o.closed = true

	// Shutdown logger provider (which flushes buffered logs)
	if err := o.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown logger provider: %w", err)
	}

	// Close gRPC connection
	if o.conn != nil {
		if err := o.conn.Close(); err != nil {
			return fmt.Errorf("failed to close gRPC connection: %w", err)
		}
	}

	return nil
}

// IsClosed returns whether the exporter has been closed
func (o *OTelLogExporter) IsClosed() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.closed
}

// GetEndpoint returns the configured OTel collector endpoint
func (o *OTelLogExporter) GetEndpoint() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.endpoint
}
