window.BENCHMARK_DATA = {
  "lastUpdate": 1725627259367,
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
        "date": 1725627259101,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 576.8876302905202,
            "unit": "MiB/s"
          },
          {
            "name": "write_10GB",
            "value": 1446.2696293564634,
            "unit": "MiB/s"
          },
          {
            "name": "write_100GB",
            "value": 1976.7200582851538,
            "unit": "MiB/s"
          },
          {
            "name": "write_40GB",
            "value": 1900.4942433960125,
            "unit": "MiB/s"
          },
          {
            "name": "read_1GB",
            "value": 1434.923148374983,
            "unit": "MiB/s"
          },
          {
            "name": "read_10GB",
            "value": 2021.4640693164324,
            "unit": "MiB/s"
          },
          {
            "name": "read_100GB",
            "value": 1950.0803550434175,
            "unit": "MiB/s"
          },
          {
            "name": "read_40GB",
            "value": 1914.5356601086812,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}