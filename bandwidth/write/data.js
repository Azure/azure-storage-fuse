window.BENCHMARK_DATA = {
  "lastUpdate": 1711112429315,
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
        "date": 1711103796015,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1003.4905598958334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1033.6041666666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1392.1643880208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 980.236328125,
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
        "date": 1711112428926,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1019.2913411458334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1061.3564453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1417.5953776041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 973.9781901041666,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}