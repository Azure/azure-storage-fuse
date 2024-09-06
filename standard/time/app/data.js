window.BENCHMARK_DATA = {
  "lastUpdate": 1725627260711,
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
        "date": 1725627260480,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.788909912109375,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 7.085815668106079,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 51.807032346725464,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 21.556497812271118,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7192022800445557,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 5.0695929527282715,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 52.5147590637207,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 21.39840006828308,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}