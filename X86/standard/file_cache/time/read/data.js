window.BENCHMARK_DATA = {
  "lastUpdate": 1768154959714,
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
        "date": 1768108806036,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.460735228498,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.928481692412,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.8679757108576668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.254561578106,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.9745988423006665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.2933306145776668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1.0674669920546667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.520467980106,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 1.305248551118,
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
        "date": 1768134223918,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.2860204902676666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 2.022828994432667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.9097355157023334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.42749434013366666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.9852071332633333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.2367720833916667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1.015384000948,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.5041084509969997,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 1.3114295869026666,
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
        "date": 1768154959472,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.0550846556756668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.6935239383996665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.9086193591366668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.25108662610533333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.2516234761240002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.744530525919,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.8084076005196668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8413041065603334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.8077415611553334,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}