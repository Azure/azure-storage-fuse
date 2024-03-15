window.BENCHMARK_DATA = {
  "lastUpdate": 1710500000853,
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
          "id": "9655fa496a1d7e09bd207e849d699f9475bf1010",
          "message": "Make write the last test case",
          "timestamp": "2024-03-15T15:28:45+05:30",
          "tree_id": "ca4ee27f4eac8d5db3140c08802b3d5984e8d09f",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9655fa496a1d7e09bd207e849d699f9475bf1010"
        },
        "date": 1710500000425,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 17547.530598958332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19665.823567708332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 807.89453125,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}