window.BENCHMARK_DATA = {
  "lastUpdate": 1711110383320,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "vibhansa@microsoft.com",
            "name": "vibhansa",
            "username": "vibhansa-msft"
          },
          "committer": {
            "email": "vibhansa@microsoft.com",
            "name": "vibhansa",
            "username": "vibhansa-msft"
          },
          "distinct": true,
          "id": "5883ec22f417b4d5fd9fd4c6499075aa349ca141",
          "message": "Add sudo to list and delete code",
          "timestamp": "2024-03-22T14:43:56+05:30",
          "tree_id": "9c816d00ef617f69ab0c306d7a0431f1e59f3953",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5883ec22f417b4d5fd9fd4c6499075aa349ca141"
        },
        "date": 1711101751101,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2.6211366300439995,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.007853486881,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.29106248481266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.5820595823216667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2.629260067396333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.20648714150134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3.8965510887243333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 5.489471247097334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 64.269054364864,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "vibhansa@microsoft.com",
            "name": "vibhansa",
            "username": "vibhansa-msft"
          },
          "committer": {
            "email": "vibhansa@microsoft.com",
            "name": "vibhansa",
            "username": "vibhansa-msft"
          },
          "distinct": true,
          "id": "7ac0753f8f1525e7f3e7060acc92944317b91797",
          "message": "Trying to correct list status",
          "timestamp": "2024-03-22T15:58:47+05:30",
          "tree_id": "46607f3062deec168aba3ac182c9b87ab9bd354b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7ac0753f8f1525e7f3e7060acc92944317b91797"
        },
        "date": 1711110382989,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8670886372623332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.67510555158866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.397590830633,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.722283534516,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.8984981400346668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.26982306484534,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2.2959292479756663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4.216588724726667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.969704886452,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}