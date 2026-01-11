window.BENCHMARK_DATA = {
  "lastUpdate": 1768164459058,
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
        "date": 1768118916854,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.821992566514,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.261365999403,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.730476810657,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.33105328759633335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.7389156549829999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.7411434533653333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.6955713558573334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.6819029766110001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.6349262951863333,
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
        "date": 1768143945515,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.889443853898,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.4091498374206666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.6993485774089999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.3371310403773333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.7821718712903333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.740090915057,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.5918671667343333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.6237640076896667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.6365324603129999,
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
        "date": 1768164458827,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 0.8359112379603332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.2724226699033334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.7652617641133332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.3312379284513334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.7186066064456668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.6087853251113334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.6614594016936667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.665912386277,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.610225014772,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}