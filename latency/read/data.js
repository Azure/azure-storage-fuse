window.BENCHMARK_DATA = {
  "lastUpdate": 1718019287307,
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
        "date": 1712228210476,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09237264668933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.102910703463,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.092863626107,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.13744655439099998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09200917992699999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.49075391474766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17470297946866667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9887939912203335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.59788254316534,
            "unit": "milliseconds"
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
        "date": 1712246931417,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10414171725733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.529268732636,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.099086034998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.21138560931633332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09646497864033332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.81312104925601,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18145079937999997,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0453324132596666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.48258914304967,
            "unit": "milliseconds"
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
        "date": 1712680823817,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09788024972299998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.24777028490766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09247800207700001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.139429245328,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.14431907473733332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 75.04025966577666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17881418975166666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0265282907916666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.31610818890034,
            "unit": "milliseconds"
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
        "date": 1712745631770,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09446754568433331,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.75949958607266,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08494615160133334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18916874891799998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09755476761433335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.13836244849433,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.183623352178,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1244443464606666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.599374912259,
            "unit": "milliseconds"
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
        "date": 1712766184779,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08724384499266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 77.18014809629267,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07878921355933334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 100061.11493275037,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10929362688366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.96152836612232,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.163633364646,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0757995910443332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.65236812201567,
            "unit": "milliseconds"
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
        "date": 1712768099262,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09665735425866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.75682781254034,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.0801089621,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1698713534013333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.108908229877,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.64738859553701,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.164786906343,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0592414714003333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.87600935867533,
            "unit": "milliseconds"
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
        "date": 1712811243941,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10604306100066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.81172013647232,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08422935547999999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18537208799466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09739952111466665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.40191478443866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17886957868533335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0587064786066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.085889302576,
            "unit": "milliseconds"
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
        "date": 1712819451268,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10111188273866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.39295319817568,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06828824501066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1776791976686667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11075022919466666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.31029072894067,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.22673813546066665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0525957113186666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.974848216425,
            "unit": "milliseconds"
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
        "date": 1712828841656,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08811265828800001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.85534154624499,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09098497316633332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17888322413299998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.099883821523,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.476661796942,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16869838031499998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9910552016973334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.36674987178033,
            "unit": "milliseconds"
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
        "date": 1712987972700,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.11026216888366665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.57678799240666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.072995219508,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19755222336833334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11113738930366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.38119898610869,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.20247739874266668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1072681177723334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.15986515342632,
            "unit": "milliseconds"
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
        "date": 1713002568104,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09909228853066666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.56741252969668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06284275692466668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.189608874061,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09051787821933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.08378903886866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.19231776830766667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0538917281776667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.764252687984,
            "unit": "milliseconds"
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
        "date": 1717669141675,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09656418027133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.30084296405066,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08313527648133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16405657743933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09856147664866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.23358800659135,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17293832626199998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0760620874393334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.66784630738165,
            "unit": "milliseconds"
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
        "date": 1717670160172,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.084393456727,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.89995981700034,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08783086908533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16788503629599996,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.101749567511,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.914768304163,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17762756442933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.080507527686,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.24338238962066,
            "unit": "milliseconds"
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
        "date": 1717672479839,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10658995562599999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 68.68337400661234,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09569025025233334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.13846967810400002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09494750835166665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.85792366566267,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.174755906988,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.083651664777,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.50178857421001,
            "unit": "milliseconds"
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
        "date": 1718005622768,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.097956909684,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.42171741060433,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.085306320187,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.2057930889023333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09893958730766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.76562742351834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17076758747666668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1358727855813333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.08206957289266,
            "unit": "milliseconds"
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
        "date": 1718013429335,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.099350838976,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.52736120078299,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.088813753787,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.216960097616,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10083916836766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.87042539111035,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.15957498278266666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0127818482646667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.26323579252367,
            "unit": "milliseconds"
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
        "date": 1718019286947,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10357550380466667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.298693756022,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.10345154220100002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.190382733602,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09807643646533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.222027314881,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.181641113306,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1087607492296667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.24427601425067,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}