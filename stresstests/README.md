# blobfuse stress and perf tests
This is designed to be a benchmark that we can use to evaluate blobfuse performance - to detect perf regressions pre-release, for example.
It's designed to be as repeatable as possible, albiet with some work.  Hopefully, these numbers can give some insight on what people can expect from blobfuse, performance-wise.

## Notes
We use the term "stress" to refer to "running a high load test and ensuring that data does not get corrupted and results are what we expect."  "Perf" refers to the latency / throughput of an operation or series of operations.  In practice, we use them interchangeably, because the current tests check perf and validate correctness in the same test run.

## Data
For the moment, we will store results of perf test runs directly in Github.  Each perf test run should contain not only the perf results (output of the blobfusestress tool), but also any information required to reproduce (system configuration, blobfuse configuration, commit ID from which to build blobfuse and blobfusestress, etc).  We can improve on this if necessary.

## Caveats
It is not always obvious how file system operations will translate into FUSE calls, and the differences between different implementations can have a large impact on performance.
For example, running the command "echo 'a' > a.txt" in a shell, in a blobfuse mounted directory, results in the following calls into FUSE before control returns to the shell:

get_attr (get attributes of the file - results in a HEAD call to the Azure Storage Service, which returns a 404 because the file 'a,txt' doesn' texist yet)
create (creates a new file in disk, does not make a REST call)
get_attr (does not make a REST call because the file is in the local cache)
write (also no rest call)
flush (makes a REST call, uploads the file to the blob)
flush (makes another REST call, uploads the file to the blob again, unsure why this happens twice)

Even though only one REST call is actually needed here (the actual upload, in the first flush()), we end up making three, due to either the way that redirection works in bash, or possibly how the FUSE driver in the kernel decides to translate syscalls to blobfuse calls, etc.  For small files like this, latency / performance is roughly proportional to the number of REST calls made.  If your file-operation library is even more verbose (calling get_attr after the final flush, for example), performance may be worse accordingly.  (Imagine if the file is created, closed, then re-opened for writing - that's even more REST calls.)  There are things we can do in the blobfuse codebase to improve this (for example, ignore subsequent flush() operations if the data hasn't changed), but until these features are implemented, performance may vary widely, even for simple operations.

Performance may also vary widely with system configurations.  For example, a 2-core machine will likely see much lower perf than a 16-core machine, for a very large workload.  Communicating with a storage account that's not co-located in the same data center as the source of the data transfers will also add significant latency to every operation.  This is why we record not only perf results, but also setup for running the perf tests

Performance may also vary widely with parallelism.  blobfuse is optimized if large jobs are run in parallel, but commands such as 'cp' don't offer this functionality.  (This is, in part, the purpose of the 'blobcp' tool, included with blobfuse.)  Optimizations for code that runs against a local disk may be detrimental to blobfuse, and vice versa.  ('stat', for example, is much cheaper on a local disk than with blobfuse, although there is room for improvement in the implementation via caching.)

## Limitations
There's a lot of room for improvement with these perf tests.  For example:

### Usability
- Perf test config is all hard-coded in the test code, instead of read from a config file
- Output is unstructured
- Results files are unstructured

### Effectiveness
We have a test for very large files and a test for very small files, but there are many other scenarios we should also test:
- Mix of large and small files
- Multiple processes reading & writing to/from the sam efile simultaneously
- Running standard file system benchmarks.  This ends up being non-trivial, due to differences between blobfuse and a fully POSIX-compliant system, but we should run and report when possible.  This will give us a different view of performance than the tests here, because these tests are specifically designed for the scenarios for which blobfuse is optimized.
