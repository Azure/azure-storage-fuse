window.BENCHMARK_DATA = {
  "lastUpdate": 1725621283503,
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
        "date": 1725621283254,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.1806640625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 105.841796875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.10546875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}