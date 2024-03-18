window.BENCHMARK_DATA = {
  "lastUpdate": 1710746899775,
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
          "id": "3f6a9fe604bef52d7cb679a7d108023e4336708c",
          "message": "Silent the clogs in case of creation",
          "timestamp": "2024-03-18T12:04:44+05:30",
          "tree_id": "9c2ea968b5fa3bff2feea572aa84a9cba678b72c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3f6a9fe604bef52d7cb679a7d108023e4336708c"
        },
        "date": 1710746899339,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 17917.976888020832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18009.273111979168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 804.9140625,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}