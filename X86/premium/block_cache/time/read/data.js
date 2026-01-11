window.BENCHMARK_DATA = {
  "lastUpdate": 1768152392567,
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
        "date": 1768106096203,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.33963569861466664,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 68.032477906382,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.24426832169133336,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.21409361671966667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.30333311027133336,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.60697124639701,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.4872722064456667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.685868877991,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.65548987309033,
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
        "date": 1768131619059,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.339441313471,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 74.24016115085699,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.24961516188666666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.298450757915,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.29511407182800004,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.24794801387401,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.479224279009,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.028211377369667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.77871320918233,
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
        "date": 1768152392323,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.3516265122803333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 71.21576086713833,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.23877673274933334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.2735808183983333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.3002892038026667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.72956968866933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.4150502162383333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.6319786926266666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.465354459808,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}