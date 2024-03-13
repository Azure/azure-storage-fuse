window.BENCHMARK_DATA = {
  "lastUpdate": 1710346854469,
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
          "id": "745533b77e66116f53b500f960152562b69910e5",
          "message": "restart with fresh data",
          "timestamp": "2024-03-13T18:34:49+05:30",
          "tree_id": "a6e4530290bd04a32a7ab40e8a86830558ef55c6",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/745533b77e66116f53b500f960152562b69910e5"
        },
        "date": 1710337059367,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2191.3157552083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 24.653971354166668,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2073.763671875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1281.7125651041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2637.1780598958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 25.644205729166668,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 4181.7548828125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 3762.1634114583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 1703.7317708333333,
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
          "id": "99b6c444486442642be5dbf7574726c66de60140",
          "message": "Recreating files",
          "timestamp": "2024-03-13T19:54:02+05:30",
          "tree_id": "201be935c384e13f01f4724e67a7db27fd458234",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/99b6c444486442642be5dbf7574726c66de60140"
        },
        "date": 1710346853283,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 518.1285807291666,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 13.073567708333334,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2338.5100911458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1083.8411458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 531.8655598958334,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 13.011393229166666,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 1944.7731119791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4782.450520833333,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_four_threads",
            "value": 51.470377604166664,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}