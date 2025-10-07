```go
// filename: atomic_test.go

func TestAtomicTestBitUint64(t *testing.T) {
    var val uint64 = 0b1101 // binary representation of 13
    addr := &val

    // Test for a bit that is set
    if !AtomicTestBitUint64(addr, 2) {
        t.Errorf("Expected bit 2 to be set.")
    }

    // Test for a bit that is not set
    if AtomicTestBitUint64(addr, 1) {
        t.Errorf("Expected bit 1 to not be set.")
    }

    // Test for an out of range bit
    defer func() {
        if r := recover(); r == nil {
            t.Errorf("Expected panic for out of range bit.")
        }
    }()
    AtomicTestBitUint64(addr, 64) // should panic
}
```