```go
// filename: parallel_writer_test.go

import (
	"errors"
	"testing"
)

func TestEnqueuDcacheWrite(t *testing.T) {
	pw := newParallelWriter()
	defer pw.destroyParallelWriter()

	var receivedErr error
	errChan := make(chan error)

	// Test case for successful write
	pw.EnqueuDcacheWrite(func() error {
		return nil
	})

	// Test case for write with error
	pw.EnqueuDcacheWrite(func() error {
		return errors.New("dcache write error")
	})

	go func() {
		receivedErr = <-errChan
	}()

	pw.dcacheWriterQueue <- &writeReq{
		write: func() error {
			return errors.New("dcache write error")
		},
		err: errChan,
	}

	if receivedErr == nil || receivedErr.Error() != "dcache write error" {
		t.Errorf("expected dcache write error, got %v", receivedErr)
	}
}
```