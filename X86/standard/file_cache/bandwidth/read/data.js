window.BENCHMARK_DATA = {
  "lastUpdate": 1768154958363,
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
        "date": 1768108803440,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 346.2171223958333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 150.1904296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1126.8916015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3928.2574869791665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 449.9807942708333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 182.87858072916666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1198.9235026041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1041.0472005208333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 779.0777994791666,
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
        "date": 1768134221498,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 355.2506510416667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 119.06901041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 744.0377604166666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 2333.27734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 369.7724609375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 174.53971354166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1265.9869791666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1040.0113932291667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 747.3434244791666,
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
        "date": 1768154956930,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 747.7262369791666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 322.3736979166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1098.0146484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3962.4453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 932.1868489583334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 738.1041666666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4069.7483723958335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 13579.0703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 3548.5436197916665,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}