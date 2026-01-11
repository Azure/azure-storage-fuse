window.BENCHMARK_DATA = {
  "lastUpdate": 1768159316301,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "id": "bff4bcf063db1d95d3f8a7ba10b498226ce1afec",
          "message": "modify benchmarks",
          "timestamp": "2026-01-09T07:33:48Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/bff4bcf063db1d95d3f8a7ba10b498226ce1afec"
        },
        "date": 1768113157485,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 712.3649088541666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 227.38118489583334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1125.0400390625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3087.65234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 666.4108072916666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 451.740234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3692.734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 18246.357096354168,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 2800.0413411458335,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "id": "e3a08c0649fd263abfb4746f0f7629695f8450d0",
          "message": "modify benchmarks",
          "timestamp": "2026-01-09T10:09:46Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e3a08c0649fd263abfb4746f0f7629695f8450d0"
        },
        "date": 1768138436374,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 697.5865885416666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 222.1591796875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1070.953125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 2849.1044921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 639.9563802083334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 391.3756510416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3719.9212239583335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 18274.776041666668,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 2604.0807291666665,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "syeleti-msft",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "446f5da15149304940ed01d95637d2e3d035fe16",
          "message": "Remove getting size from statfs (#2083)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>",
          "timestamp": "2026-01-09T09:42:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/446f5da15149304940ed01d95637d2e3d035fe16"
        },
        "date": 1768159316058,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1086.6396484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 506.79296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1095.3736979166667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 2891.4143880208335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1092.3072916666667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1064.619140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4720.196940104167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 18992.771809895832,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 4528.147135416667,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}