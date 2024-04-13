window.BENCHMARK_DATA = {
  "lastUpdate": 1712987973001,
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
      }
    ]
  }
}