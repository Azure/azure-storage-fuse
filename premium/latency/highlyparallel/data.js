window.BENCHMARK_DATA = {
  "lastUpdate": 1725619648822,
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
        "date": 1725619648595,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 305.4540478942967,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 407.453109737577,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1704.2583711127593,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}