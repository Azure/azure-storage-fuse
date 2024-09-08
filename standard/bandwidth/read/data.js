window.BENCHMARK_DATA = {
  "lastUpdate": 1725774213125,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "syeleti@microsoft.com",
            "name": "syeleti-msft",
            "username": "syeleti-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "a42da599c0d65e83577b4235ef0e581e68bd06b3",
          "message": "Making pipeline consistent with the units (#1461)",
          "timestamp": "2024-09-06T15:37:46+05:30",
          "tree_id": "dec09c4da4dcd8fa7bdf1922c2214e2050ee087e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a42da599c0d65e83577b4235ef0e581e68bd06b3"
        },
        "date": 1725623462346,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2410.2106119791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 1.9710286458333333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2769.0823567708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1208.2399088541667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2566.1526692708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 2.166015625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4718.705729166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3727.4720052083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 8.501953125,
            "unit": "MiB/s"
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
          "id": "a42da599c0d65e83577b4235ef0e581e68bd06b3",
          "message": "Making pipeline consistent with the units (#1461)",
          "timestamp": "2024-09-06T10:07:46Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a42da599c0d65e83577b4235ef0e581e68bd06b3"
        },
        "date": 1725774212053,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2234.1780598958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 2.072265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2759.5354817708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1611.6705729166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2300.1341145833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 2.18359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4798.172526041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3623.4619140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 8.494466145833334,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}