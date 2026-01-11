window.BENCHMARK_DATA = {
  "lastUpdate": 1768149923323,
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
        "date": 1768103618626,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.5024643161166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 2.127287810482,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.9638722725056666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.268984925224,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.8474050082546666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.4342285589193333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1.1551931150123333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.3009252351523335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 1.2374739347663333,
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
        "date": 1768129400205,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.486522018631,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 2.0579208431876665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.9509287627226666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.3009689422943333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.004361428333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 1.2224947973183333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1.0947590247776668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.5122683782993334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 1.2407735554523334,
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
        "date": 1768149923066,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read_kernel_cache",
            "value": 1.5195806459053334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_kernel_cache",
            "value": 1.431410113096,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.6804975211830001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.24848421782533334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.0063290200496666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 0.6655674333673334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.7488838365530001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9281292289743334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 0.9283998168246668,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}