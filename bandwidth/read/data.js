window.BENCHMARK_DATA = {
  "lastUpdate": 1718950844816,
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
          "id": "1a4e554337ce8799974951b862bc67522031adf1",
          "message": "Correcting bs in large write case",
          "timestamp": "2024-04-04T15:58:04+05:30",
          "tree_id": "4a43bfe9042ae83dab8725a2ba1ea42ed150b950",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a4e554337ce8799974951b862bc67522031adf1"
        },
        "date": 1712228208615,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2482.5139973958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6279296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2336.6272786458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1649.4833984375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2708.4593098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4537760416666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4761.986979166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3977.8662109375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.228515625,
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
          "id": "af6c4b7f5027b190ed6fd22b9411c121cde95161",
          "message": "Sync with main",
          "timestamp": "2024-04-04T21:19:24+05:30",
          "tree_id": "8d44ebd434f1348fa4eccb4120623d585257ea2e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/af6c4b7f5027b190ed6fd22b9411c121cde95161"
        },
        "date": 1712246929661,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2186.5791015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4495442708333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2211.4801432291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1105.3404947916667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2586.2327473958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.38671875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4783.632161458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3722.6256510416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.223958333333334,
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
          "id": "efc39a9a7a9ade6bef2ade06f5134a61ca3708c8",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/perftestrunner",
          "timestamp": "2024-04-09T21:50:08+05:30",
          "tree_id": "919ec536002591c79c706b99acb15eccd3353c73",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/efc39a9a7a9ade6bef2ade06f5134a61ca3708c8"
        },
        "date": 1712680821983,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2345.607421875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4596354166666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2386.4059244791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1628.9202473958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1809.7392578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3313802083333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4668.119140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3796.8642578125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.612955729166666,
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
          "id": "2dbf6d58c1321a1f4bbe717f34f74bfed3983457",
          "message": "Updated",
          "timestamp": "2024-04-10T15:50:02+05:30",
          "tree_id": "a011193a4c059ca872fde238f30b693f3cbbd3ce",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/2dbf6d58c1321a1f4bbe717f34f74bfed3983457"
        },
        "date": 1712745629722,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2391.1087239583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.5846354166666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2555.4449869791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1226.9000651041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2563.1975911458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5719401041666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4561.980143229167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3493.0120442708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.377604166666666,
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
          "id": "c0b0c7080e461377d2333ac1a44a32ee94ba6578",
          "message": "Add more logs",
          "timestamp": "2024-04-10T21:29:14+05:30",
          "tree_id": "85dc2b3d9872cab72549fd5303f2595e131c3f3d",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c0b0c7080e461377d2333ac1a44a32ee94ba6578"
        },
        "date": 1712766182909,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2571.8304036458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.2412109375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2825.6110026041665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 770.732421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2282.6569010416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5231119791666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4998.3857421875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3623.8912760416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.603190104166666,
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
          "id": "c0b0c7080e461377d2333ac1a44a32ee94ba6578",
          "message": "Add more logs",
          "timestamp": "2024-04-10T21:29:14+05:30",
          "tree_id": "85dc2b3d9872cab72549fd5303f2595e131c3f3d",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c0b0c7080e461377d2333ac1a44a32ee94ba6578"
        },
        "date": 1712768097240,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2366.8011067708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4401041666666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2677.107421875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1400.6891276041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2289.2317708333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4016927083333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5037.4921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3686.1565755208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.777669270833334,
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
          "id": "5f34c4977e7888a185cc80edabadd14cdcba9286",
          "message": "app results correction",
          "timestamp": "2024-04-11T10:04:10+05:30",
          "tree_id": "3484c94e4dc7110aecb86d40bb89f89380e5e8c7",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5f34c4977e7888a185cc80edabadd14cdcba9286"
        },
        "date": 1712811241996,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2150.1168619791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.58203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2578.283203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1288.5537109375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2566.1083984375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3610026041666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4726.6318359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3671.7154947916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.6591796875,
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
          "id": "ab429f5df97a6ccd09a850be782c34bacfd1c00f",
          "message": "Correcting result path",
          "timestamp": "2024-04-11T12:19:53+05:30",
          "tree_id": "f8b81414d6d3a440fdf894894b6ff52f61d5fb0b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ab429f5df97a6ccd09a850be782c34bacfd1c00f"
        },
        "date": 1712819449378,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2252.791015625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6038411458333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3052.7005208333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1336.046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2253.7688802083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4586588541666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4116.34375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3692.8727213541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.312825520833334,
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
          "id": "c81e8b6a4252e2ffcf97166599adc92ef7c3c2c1",
          "message": "Add bandiwdth and times for application tests",
          "timestamp": "2024-04-11T14:57:44+05:30",
          "tree_id": "c0328cc59b8267b5cc2ec66f6e64cb29d56759af",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c81e8b6a4252e2ffcf97166599adc92ef7c3c2c1"
        },
        "date": 1712828839731,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2540.8896484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.5302734375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2437.5654296875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1292.7203776041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2511.0908203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5504557291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4976.215494791667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3917.3603515625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.259765625,
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
          "id": "98713b84de33423d69095a1d6bb70cdef931f280",
          "message": "Adding local app writing",
          "timestamp": "2024-04-13T11:08:46+05:30",
          "tree_id": "082bb4a0552af493923454ceb93dbb6564932e6d",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98713b84de33423d69095a1d6bb70cdef931f280"
        },
        "date": 1712987970913,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2073.9524739583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.5595703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2911.2776692708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1182.7044270833333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2251.720703125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5517578125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4412.860026041667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3541.9921875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.11328125,
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
          "id": "f2ae5860da5bf4297a46e89e56526ec8d97637fe",
          "message": "Correcting output format",
          "timestamp": "2024-04-13T15:12:42+05:30",
          "tree_id": "d4b212c9c3d54a787a003056dba956ac93666217",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f2ae5860da5bf4297a46e89e56526ec8d97637fe"
        },
        "date": 1713002566231,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2332.7467447916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4443359375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3265.646484375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1275.529296875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2753.0491536458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.6184895833333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4542.3134765625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3713.9772135416665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.042643229166666,
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
          "id": "8820477da3584b1bcc92084fa79ae9de276d45ed",
          "message": "Adding parallel read/write scripts",
          "timestamp": "2024-06-06T02:58:30-07:00",
          "tree_id": "30d380ffe1bd809dc9838543a838efad09638ee3",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/8820477da3584b1bcc92084fa79ae9de276d45ed"
        },
        "date": 1717669139523,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2371.5677083333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4593098958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2636.3616536458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1429.9658203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2530.5374348958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5628255208333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4857.344075520833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3597.4514973958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.544596354166666,
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
          "id": "b909fee53ee26c30408d47ca08cdea0eac89dc30",
          "message": "correcting files",
          "timestamp": "2024-06-06T03:15:30-07:00",
          "tree_id": "42b09e487fdf6a0491919bb1e65e25e8abd37224",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/b909fee53ee26c30408d47ca08cdea0eac89dc30"
        },
        "date": 1717670158162,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2637.1282552083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.3505859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2524.83203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1390.2057291666667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2454.3649088541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3821614583333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4726.638997395833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3608.7376302083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.628255208333334,
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
          "id": "570b0653ad8667ce4a99bdc321d04f614c428b05",
          "message": "adding json package to script",
          "timestamp": "2024-06-06T03:55:33-07:00",
          "tree_id": "572fc44e7a5ad70ead5ee61ab3768dec280a24ff",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/570b0653ad8667ce4a99bdc321d04f614c428b05"
        },
        "date": 1717672477910,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2134.5338541666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.642578125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2271.2874348958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1635.55859375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2629.1481119791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.533203125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4805.009440104167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3602.8108723958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.394856770833334,
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
          "id": "2304450fd2ad0c8fc8b813af8f7d10b19d489a71",
          "message": "Correcting script",
          "timestamp": "2024-06-10T00:26:55-07:00",
          "tree_id": "ec05d5bb32910920d836f5e27e44bf608cd682ef",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/2304450fd2ad0c8fc8b813af8f7d10b19d489a71"
        },
        "date": 1718005620764,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2315.1643880208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4069010416666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2551.294921875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1130.6396484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2523.85546875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4830729166666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4803.836263020833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3446.5045572916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.474609375,
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
          "id": "98a73483767ee112f91904d1ddbf7d64842980ba",
          "message": "correcting list test case:",
          "timestamp": "2024-06-10T02:36:07-07:00",
          "tree_id": "5acf15ff53416eb666947ff82504f760aa82bf37",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98a73483767ee112f91904d1ddbf7d64842980ba"
        },
        "date": 1718013427209,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2274.056640625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4521484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2513.6204427083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1075.6471354166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2505.2744140625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4788411458333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5186.595703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3841.8815104166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.2626953125,
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
          "id": "34fb8795e91188fbb10dabcab800920f5bbab5a4",
          "message": "Adding rename test",
          "timestamp": "2024-06-10T04:14:35-07:00",
          "tree_id": "8e971874f83d977053a5f930e181b6410a344a29",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/34fb8795e91188fbb10dabcab800920f5bbab5a4"
        },
        "date": 1718019284629,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2186.4407552083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6064453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2111.6871744791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1220.0725911458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2546.4811197916665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.6110026041666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4689.369140625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3547.4612630208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.266927083333334,
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
          "id": "f6ebe28bc5b6719bec2f74563bf8334eecdad711",
          "message": "Correcting logs",
          "timestamp": "2024-06-11T02:24:38-07:00",
          "tree_id": "02627ecb7c295448ce45c5b045f5758de099c34b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f6ebe28bc5b6719bec2f74563bf8334eecdad711"
        },
        "date": 1718099133234,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2224.6363932291665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6129557291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2746.533203125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1434.2932942708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2294.984375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5472005208333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4671.3115234375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3610.2333984375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.684895833333334,
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
          "id": "2f1f593eba3a34df514f03a3d25869c627bc52cf",
          "message": "Sync with main",
          "timestamp": "2024-06-18T02:59:37-07:00",
          "tree_id": "fbf24e306fa6c5464d3dce0e45c3fe64d9617aeb",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/2f1f593eba3a34df514f03a3d25869c627bc52cf"
        },
        "date": 1718705997320,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2301.9169921875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.51171875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2232.7252604166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1099.3821614583333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2293.5843098958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5572916666666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4781.0771484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3597.9072265625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.5,
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
          "id": "f0fc0d0f599970d29defdab892c5a2e676566295",
          "message": "Updated path",
          "timestamp": "2024-06-18T09:47:59-07:00",
          "tree_id": "79876105e05171f00801174f62b0123fae15d1f6",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f0fc0d0f599970d29defdab892c5a2e676566295"
        },
        "date": 1718730503602,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2383.8701171875,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4244791666666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2867.4899088541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1195.7542317708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2688.1292317708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.3629557291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4923.7138671875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3745.9990234375,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 12.642903645833334,
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
          "id": "760c3d4cf0c310292f2f6fea2d71acdc6fdc9e24",
          "message": "Updated",
          "timestamp": "2024-06-18T22:35:50-07:00",
          "tree_id": "e6fc3a7e6e9fff003a7232db5e44fd70b3a23a8b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/760c3d4cf0c310292f2f6fea2d71acdc6fdc9e24"
        },
        "date": 1718776572348,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2309.0696614583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.5498046875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2726.1722005208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1219.064453125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2431.0130208333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.4641927083333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4663.502604166667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3659.1458333333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.197591145833334,
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
          "id": "e9d900e661f239e88aca4ebf68735f7c625a23bd",
          "message": "Updating script",
          "timestamp": "2024-06-19T00:25:31-07:00",
          "tree_id": "24cf4f783f0894e9b275c8d840b84efbabdb89af",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e9d900e661f239e88aca4ebf68735f7c625a23bd"
        },
        "date": 1718783122961,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2269.5071614583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.45703125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2776.3206380208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1356.2822265625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2556.6002604166665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.646484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4722.8603515625,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3743.53515625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.834635416666666,
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
          "id": "0f894c71375cad64d5681d7d97b8880cac141919",
          "message": "Updating rename results",
          "timestamp": "2024-06-19T01:12:36-07:00",
          "tree_id": "6fca09c905e96f82b6aa174f5fbfdcc464ea2569",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0f894c71375cad64d5681d7d97b8880cac141919"
        },
        "date": 1718789820067,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2302.3255208333335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.6598307291666665,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2763.7822265625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1605.4514973958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2254.4798177083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.6214192708333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 5043.819010416667,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3597.1455078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.710286458333334,
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
          "id": "5438da33f6f0e094598653070ded1f5e003dd4d0",
          "message": "Seperate out highspeed outputs",
          "timestamp": "2024-06-19T03:29:11-07:00",
          "tree_id": "e8649608dc41f62b2e663a9bc4ad75d308db6831",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5438da33f6f0e094598653070ded1f5e003dd4d0"
        },
        "date": 1718794091904,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2145.3001302083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4710286458333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 3056.90625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1275.0403645833333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2633.7727864583335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.6663411458333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4798.575846354167,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3571.4977213541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.468424479166666,
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
          "id": "f527f1886232518df3f7f6fb7915308d8d3780e7",
          "message": "Updated",
          "timestamp": "2024-06-19T22:30:00-07:00",
          "tree_id": "49bf4364f3c8cb6fb179531a8572c458d06f9176",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f527f1886232518df3f7f6fb7915308d8d3780e7"
        },
        "date": 1718862610544,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2293.0260416666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.4729817708333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2450.1761067708335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1434.7991536458333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2227.7268880208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5257161458333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4624.639973958333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3830.462890625,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.298177083333334,
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
          "id": "cf65fd86fb159a9e7232c4420c63bcc9318f876e",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/perftestrunner",
          "timestamp": "2024-06-20T22:59:46-07:00",
          "tree_id": "8132c55336cdf1dde4ed6875eeb8dcf1472e55ed",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/cf65fd86fb159a9e7232c4420c63bcc9318f876e"
        },
        "date": 1718950843631,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2277.923828125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.669921875,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2700.6263020833335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1207.6370442708333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2578.1930338541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.5764973958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4726.471028645833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3591.8128255208335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 13.224609375,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}