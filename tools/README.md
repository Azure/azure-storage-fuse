# blobfuse helper tools
## blobcp - parallel copy tool

blobcp is a parallel copy tool to help optimize transferring data between file systems, specifically from/to blobfuse mount. Unlike cp tool on Linux, blobcp transfers data concurrently which increases the overall throughput.

### Installation

blobcp is installed with blobfuse. You can find the binary at /usr/local/bin/blobcp or /usr/bin/blobcp.

### Examples

#### Copy data between folders

```blobcp /path/to/source /path/to/destination```

#### Copy data between folders with a filter on source

```blobcp /path/to/source /path/to/destination --pattern "*.png"```

#### Change the concurrency

```blobcp /path/to/source /path/to/destination -n 128```
