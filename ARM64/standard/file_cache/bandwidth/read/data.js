window.BENCHMARK_DATA = {
  "lastUpdate": 1768164457777,
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
        "date": 1768118913954,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 708.2740885416666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 248.09244791666666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1054.1070963541667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3008.6217447916665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 662.1979166666666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 469.5598958333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3820.8772786458335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 18391.192057291668,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 3038.55859375,
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
        "date": 1768143942818,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 712.0970052083334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 246.4599609375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1012.8977864583334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 2955.0452473958335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 710.4860026041666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 476.2600911458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4013.5266927083335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 18404.523763020832,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 2936.0068359375,
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
        "date": 1768164456241,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1108.7161458333333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 511.5026041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 1011.619140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 3006.9768880208335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1017.2744140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 1075.2392578125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4541.40625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 18869.737630208332,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 4270.7265625,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}