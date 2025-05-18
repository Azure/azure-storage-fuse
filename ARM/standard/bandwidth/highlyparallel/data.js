window.BENCHMARK_DATA = {
  "lastUpdate": 1747561730668,
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
          "id": "03e72e47d37985e5c28051c0ff17bdc0c7315e74",
          "message": "Correcting code for cache cleanup",
          "timestamp": "2025-03-07T23:48:22-08:00",
          "tree_id": "5b69276c81c0c728ae2dd3889b9743194fdcc990",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/03e72e47d37985e5c28051c0ff17bdc0c7315e74"
        },
        "date": 1741577864945,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 53.367838541666664,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 573.5068359375,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 3.3212890625,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "64532198+vibhansa-msft@users.noreply.github.com",
            "name": "Vikas Bhansali",
            "username": "vibhansa-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "79978157a7cb7035566f743a8c86becadf2dec81",
          "message": "Merge branch 'main' into vibhansa/armperftest",
          "timestamp": "2025-03-10T22:32:05+05:30",
          "tree_id": "cbd7d68b0a780722eb7ff9ee8e431fec9495a607",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/79978157a7cb7035566f743a8c86becadf2dec81"
        },
        "date": 1741644533691,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 386.1569010416667,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 83.33984375,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 17.920247395833332,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "syeleti@microsoft.com",
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft"
          },
          "committer": {
            "email": "syeleti@microsoft.com",
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft"
          },
          "distinct": true,
          "id": "abeffac3531ff852a6240abfd960d263274a1527",
          "message": "remove warning errors that is preventing the run to happen",
          "timestamp": "2025-03-19T07:21:38Z",
          "tree_id": "fb855960d36d4e3f6914668bbc2c3a116cd23e8a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/abeffac3531ff852a6240abfd960d263274a1527"
        },
        "date": 1742387179605,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 37882.5087890625,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 49491.2607421875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 806.7506510416666,
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
          "id": "643e91f6a0e3b6c677d4f89a36e8e5209e046ec6",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/armperftest",
          "timestamp": "2025-04-09T21:55:32-07:00",
          "tree_id": "44e6262b9b6c585dac173204bc04e4eb084a6e47",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/643e91f6a0e3b6c677d4f89a36e8e5209e046ec6"
        },
        "date": 1744279496632,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 40048.142252604164,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 46908.680989583336,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 854.0296223958334,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Vikas Bhansali",
            "username": "vibhansa-msft",
            "email": "64532198+vibhansa-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd",
          "message": "Adding arm based benchmark tests (#1654)\n\nCo-authored-by: Srinivas Yeleti <syeleti@microsoft.com>",
          "timestamp": "2025-04-10T15:43:25Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd"
        },
        "date": 1744358524072,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 36276.280598958336,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 50256.3203125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 785.6363932291666,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Vikas Bhansali",
            "username": "vibhansa-msft",
            "email": "64532198+vibhansa-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1744539896586,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 39028.909830729164,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 49968.689453125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 882.1067708333334,
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
          "id": "887bdba6cde3bc805787a410ea3fb4520a830392",
          "message": "Updating README for preload",
          "timestamp": "2025-04-13T23:47:01-07:00",
          "tree_id": "07eea2db5ae5f4e8cc51ae7445f1edbfbd81b581",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/887bdba6cde3bc805787a410ea3fb4520a830392"
        },
        "date": 1744634383998,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 39641.637369791664,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 49209.500325520836,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 865.3606770833334,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Vikas Bhansali",
            "username": "vibhansa-msft",
            "email": "64532198+vibhansa-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1745142323811,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 38424.742513020836,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 50080.268229166664,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 892.3610026041666,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "James Fantin-Hardesty",
            "username": "jfantinhardesty",
            "email": "24646452+jfantinhardesty@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "1667ad8b4bebf79badfccb915c351fd3209883a9",
          "message": "Feature: Lazy unmount (#1705)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>",
          "timestamp": "2025-04-26T07:11:48Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1667ad8b4bebf79badfccb915c351fd3209883a9"
        },
        "date": 1745746927601,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 39544.139973958336,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 47880.0498046875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 877.9388020833334,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746352323884,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 37914.220052083336,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 49025.0244140625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 831.052734375,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746956983685,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 38612.412434895836,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 48271.237955729164,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 851.0696614583334,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1747561730353,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 37480.212239583336,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 47254.3046875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 836.8190104166666,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}