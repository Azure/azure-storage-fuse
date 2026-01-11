window.BENCHMARK_DATA = {
  "lastUpdate": 1768157423500,
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
        "date": 1768111221985,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.409433452879,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 176.646092986551,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.23954582622099999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.22128690754933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.36413976058700004,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 178.17866939232866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.569743506643,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.835239743124,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 176.35297410339567,
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
        "date": 1768136495023,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.37646422368699994,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 180.361413719222,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.24966953072266665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.31069993673299995,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.33490812086899996,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 172.35782792051768,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.5202459098483333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.0101863112376663,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 171.676504552476,
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
        "date": 1768157423254,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.40929699700933336,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 172.20116512110334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.29437673104533335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.335343452266,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.33313925957566665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 169.928191074728,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.4803266798056667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.6506835695896667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 169.06216810098266,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}