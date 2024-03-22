window.BENCHMARK_DATA = {
  "lastUpdate": 1711110382188,
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
        "date": 1711101749226,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 373.6031901041667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 14.278971354166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2899.5234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1570.2900390625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 380.2360026041667,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.660481770833334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 983.0748697916666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2795.1819661458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 61.917317708333336,
            "unit": "MiB/s"
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
        "date": 1711110381074,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 521.1471354166666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 14.777018229166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2209.4703776041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1316.4443359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 526.5589192708334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 14.235677083333334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 1606.234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3717.1725260416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 55.299153645833336,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}