window.BENCHMARK_DATA = {
  "lastUpdate": 1768145015135,
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
        "date": 1768120270644,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write_kernel_cache",
            "value": 0.4926007928423333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.608094003484,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.6228750218286666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1.986609082494,
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
        "date": 1768145014906,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write_kernel_cache",
            "value": 17.320619128106333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 1.4219957174776667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1.5044304643300002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 2.2282915848966667,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}