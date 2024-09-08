window.BENCHMARK_DATA = {
  "lastUpdate": 1725775916819,
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
        "date": 1725625139035,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1930.052734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1827.7526041666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1741.4635416666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1944.5319010416667,
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
        "date": 1725775916512,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 1971.9404296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_directio",
            "value": 1850.9459635416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 1945.3206380208333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 1912.1673177083333,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}