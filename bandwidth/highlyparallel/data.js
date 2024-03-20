window.BENCHMARK_DATA = {
  "lastUpdate": 1710931295079,
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
          "id": "3bc15cf143c5c8ccfbe50dce3f4ee190c9a6fe02",
          "message": "Reset open files setting",
          "timestamp": "2024-03-19T15:31:59+05:30",
          "tree_id": "bcd3da2f495e1f5e3719d8bae7fb32b00f9ec479",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3bc15cf143c5c8ccfbe50dce3f4ee190c9a6fe02"
        },
        "date": 1710847662799,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 18442.24609375,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18727.951171875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 798.390625,
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
          "id": "a34ec9f46def7c71da12fb55e24f370c72219ca4",
          "message": "Remove log printing stage",
          "timestamp": "2024-03-20T12:13:40+05:30",
          "tree_id": "4ea0b229bc95797534203bd46148d3d2b1608cae",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a34ec9f46def7c71da12fb55e24f370c72219ca4"
        },
        "date": 1710920058411,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 18003.134114583332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 17897.4892578125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 744.84375,
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
          "id": "211d41ab52a667e5626c3a5c6edefb6fb368cef1",
          "message": "Correct listing command",
          "timestamp": "2024-03-20T15:26:01+05:30",
          "tree_id": "4c66641a180cc021b8a5089bc138094e7367dd5b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/211d41ab52a667e5626c3a5c6edefb6fb368cef1"
        },
        "date": 1710931294658,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 14732.31640625,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18309.644205729168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1994.9449869791667,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}