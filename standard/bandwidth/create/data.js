window.BENCHMARK_DATA = {
  "lastUpdate": 1720614562822,
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
          "id": "65a677429517c28a75a8ab6a3051311f44ba96aa",
          "message": "Revert back to parallel runs",
          "timestamp": "2024-07-02T20:58:56-07:00",
          "tree_id": "6f63f586f70486b7a8ef5ccd4a64d7862cb7715c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/65a677429517c28a75a8ab6a3051311f44ba96aa"
        },
        "date": 1719988813696,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 89.693359375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 90.341796875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.087890625,
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
          "id": "65a677429517c28a75a8ab6a3051311f44ba96aa",
          "message": "Revert back to parallel runs",
          "timestamp": "2024-07-02T20:58:56-07:00",
          "tree_id": "6f63f586f70486b7a8ef5ccd4a64d7862cb7715c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/65a677429517c28a75a8ab6a3051311f44ba96aa"
        },
        "date": 1720008092962,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 93.318359375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 96.7021484375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.087890625,
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
          "id": "65a677429517c28a75a8ab6a3051311f44ba96aa",
          "message": "Revert back to parallel runs",
          "timestamp": "2024-07-02T20:58:56-07:00",
          "tree_id": "6f63f586f70486b7a8ef5ccd4a64d7862cb7715c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/65a677429517c28a75a8ab6a3051311f44ba96aa"
        },
        "date": 1720091978277,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 100.8974609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 109.181640625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0966796875,
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
          "id": "5eabe8a1f5f8325cf880d539c15a503be4d38cb2",
          "message": "Create PerfTest.yml (#1349)\n\n* Create perf test runner to regularly benchmark performance",
          "timestamp": "2024-07-09T15:36:23+05:30",
          "tree_id": "15817378f278eacf9de12eaaa7fdcb7aff2216dc",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5eabe8a1f5f8325cf880d539c15a503be4d38cb2"
        },
        "date": 1720523936645,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 93.9931640625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 92.7470703125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.091796875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "137055338+ashruti-msft@users.noreply.github.com",
            "name": "ashruti-msft",
            "username": "ashruti-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T15:53:58+05:30",
          "tree_id": "4e14a54cfb0e3c7370afc2f0842cdbb04717c9f8",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720524224104,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 93.703125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 93.8701171875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1416015625,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T10:23:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720531216524,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 92.6611328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 96.673828125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0908203125,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T10:23:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720541673015,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 102.7958984375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 101.791015625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.099609375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T10:23:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720552552809,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 109.265625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 110.15625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0966796875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T10:23:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720563182646,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 99.6015625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 95.2099609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1015625,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T10:23:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720575173170,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 98.8330078125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 95.8955078125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0927734375,
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
          "id": "5d7a9e4a7bb4ae16ed9b50d878fa817c40b1d284",
          "message": "Update benchmark.yml",
          "timestamp": "2024-07-10T07:41:14+05:30",
          "tree_id": "de12d98c3c6c45fdf002f9cf83fcacde6303ed16",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5d7a9e4a7bb4ae16ed9b50d878fa817c40b1d284"
        },
        "date": 1720581746062,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 98.7451171875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 96.96484375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0966796875,
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
          "id": "8f767d0251fc23bddb7dc73f3a2a8e792f39412d",
          "message": "Remove RHEL 7.5 from nightly and artifacts. (#1448)\n\n* Remove RHLE7.5 from nightly and artifact tests.",
          "timestamp": "2024-07-10T07:11:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/8f767d0251fc23bddb7dc73f3a2a8e792f39412d"
        },
        "date": 1720595980051,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 98.8525390625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 95.8037109375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.087890625,
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
          "id": "8f767d0251fc23bddb7dc73f3a2a8e792f39412d",
          "message": "Remove RHEL 7.5 from nightly and artifacts. (#1448)\n\n* Remove RHLE7.5 from nightly and artifact tests.",
          "timestamp": "2024-07-10T07:11:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/8f767d0251fc23bddb7dc73f3a2a8e792f39412d"
        },
        "date": 1720606454171,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 104.7998046875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 100.4814453125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.099609375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "137055338+ashruti-msft@users.noreply.github.com",
            "name": "ashruti-msft",
            "username": "ashruti-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T16:47:29+05:30",
          "tree_id": "8a8dbaf7c359e6a5d135022861dcf3927c07497b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720614562349,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 104.0791015625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 102.1025390625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.09375,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}