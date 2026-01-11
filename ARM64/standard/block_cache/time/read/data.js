window.BENCHMARK_DATA = {
  "lastUpdate": 1768167403244,
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
        "date": 1768121843550,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.3473172227356667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 170.238744149627,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.2917636706596667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.26846526725966663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3429245718096667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 170.157207245676,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.43984044427033336,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.6229122706863333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 167.322518361546,
            "unit": "milliseconds"
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
        "date": 1768146600012,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.32924787325766663,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 168.14215292862934,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.27269698829033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.26434124722766666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3311831760483333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 160.85381473180098,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.44012368354533327,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.647801075095,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 157.66306618903002,
            "unit": "milliseconds"
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
        "date": 1768167403013,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.33974415093866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 164.41555864900198,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.2708379983813333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.28336278269066667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3425379614463333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 157.30214282248733,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.42174159078100004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.6408175601466664,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 156.49678528541565,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}