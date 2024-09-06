window.BENCHMARK_DATA = {
  "lastUpdate": 1725620077721,
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
        "date": 1725620077492,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.130695326064,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.14446202271099998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.12871238185566666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.127694337943,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}