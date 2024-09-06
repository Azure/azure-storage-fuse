window.BENCHMARK_DATA = {
  "lastUpdate": 1725621985112,
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
        "date": 1725621984884,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.2491164207458496,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.08256983757019,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 51.74950885772705,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 22.018192052841187,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7928786277770996,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.351671934127808,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 50.66012907028198,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 18.78116202354431,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}