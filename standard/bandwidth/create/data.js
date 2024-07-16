window.BENCHMARK_DATA = {
  "lastUpdate": 1721117398624,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720628252394,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 100.8056640625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 100.61328125,
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
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720639010846,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.3369140625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 102.01953125,
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
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720649653901,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 99.7001953125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 105.107421875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1005859375,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720661250972,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 96.955078125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 109.79296875,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720671603306,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 89.3330078125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 95.546875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.08984375,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720682355624,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 94.6337890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 91.9111328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0869140625,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720692973720,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 102.626953125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 111.048828125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.09375,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720703924979,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.57421875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 96.0244140625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0888671875,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720714726125,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 94.267578125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 94.4189453125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.08984375,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720725463191,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 99.32421875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 103.6259765625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.08984375,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720736069215,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 106.6552734375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 103.5087890625,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720747784638,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 95.2373046875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 96.310546875,
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
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720757866759,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 95.4013671875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 91.33203125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0859375,
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
          "id": "30b4cfff983f65c1d9a2e438386e0ae5120d4eda",
          "message": "Update benchmark.yml",
          "timestamp": "2024-07-12T12:43:58+05:30",
          "tree_id": "6070fb3ee452210ea6f5b6544582d99000a8a5fa",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/30b4cfff983f65c1d9a2e438386e0ae5120d4eda"
        },
        "date": 1720772760247,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 89.693359375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 90.46484375,
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
            "name": "Vikas Bhansali",
            "username": "vibhansa-msft",
            "email": "64532198+vibhansa-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "30b4cfff983f65c1d9a2e438386e0ae5120d4eda",
          "message": "Update benchmark.yml",
          "timestamp": "2024-07-12T07:13:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/30b4cfff983f65c1d9a2e438386e0ae5120d4eda"
        },
        "date": 1720779376409,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 106.72265625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 104.2529296875,
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
          "id": "7f395591dbea6264df3160b99c37fbaf4baea1dd",
          "message": "ObjectID info updated and simplified base config (#1452)",
          "timestamp": "2024-07-12T16:11:38+05:30",
          "tree_id": "69aa243fa590be261f16a9dae2e51e612b563b64",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7f395591dbea6264df3160b99c37fbaf4baea1dd"
        },
        "date": 1720785340274,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 93.4833984375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 93.205078125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.095703125,
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
          "id": "7f395591dbea6264df3160b99c37fbaf4baea1dd",
          "message": "ObjectID info updated and simplified base config (#1452)",
          "timestamp": "2024-07-12T10:41:38Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7f395591dbea6264df3160b99c37fbaf4baea1dd"
        },
        "date": 1720790437474,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 98.0869140625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98.134765625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0888671875,
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
          "id": "7f395591dbea6264df3160b99c37fbaf4baea1dd",
          "message": "ObjectID info updated and simplified base config (#1452)",
          "timestamp": "2024-07-12T10:41:38Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7f395591dbea6264df3160b99c37fbaf4baea1dd"
        },
        "date": 1720801122241,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 99.2353515625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98.01953125,
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
            "name": "ashruti-msft",
            "username": "ashruti-msft",
            "email": "137055338+ashruti-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "7f395591dbea6264df3160b99c37fbaf4baea1dd",
          "message": "ObjectID info updated and simplified base config (#1452)",
          "timestamp": "2024-07-12T10:41:38Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7f395591dbea6264df3160b99c37fbaf4baea1dd"
        },
        "date": 1720821971659,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 99.462890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 106.8369140625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0986328125,
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
          "id": "7f395591dbea6264df3160b99c37fbaf4baea1dd",
          "message": "ObjectID info updated and simplified base config (#1452)",
          "timestamp": "2024-07-12T10:41:38Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7f395591dbea6264df3160b99c37fbaf4baea1dd"
        },
        "date": 1720844231858,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 97.3896484375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 99.47265625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0869140625,
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
          "id": "7f395591dbea6264df3160b99c37fbaf4baea1dd",
          "message": "ObjectID info updated and simplified base config (#1452)",
          "timestamp": "2024-07-12T10:41:38Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7f395591dbea6264df3160b99c37fbaf4baea1dd"
        },
        "date": 1720855085558,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 103.9599609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 95.1201171875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.08984375,
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
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T00:34:35-07:00",
          "tree_id": "94512e2fc047f56c43a253843883d95a60917ac3",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720860463831,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 108.5419921875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 112.169921875,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720865392248,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 96.2919921875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 87.52734375,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720876635787,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 102.5849609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98.2216796875,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720887402124,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 99.126953125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 99.068359375,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720898243142,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 104.3291015625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98.6484375,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720909029281,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.111328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98.154296875,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720920687656,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 91.1904296875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 99.53125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.08984375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720930884464,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 90.603515625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 93.37890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0810546875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720941615019,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 100.765625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 100.8974609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0869140625,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720952400392,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 93.87890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 106.021484375,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720963749578,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 100.3408203125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 102.3642578125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.083984375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720974175088,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 95.310546875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 100.5224609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0869140625,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720984833155,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.822265625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 101.4404296875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0888671875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1720995397791,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 100.4814453125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 104.72265625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0947265625,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721007161359,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 97.8369140625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 93.791015625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0869140625,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721017183056,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 93.3701171875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 95.12890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0830078125,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721028063925,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 93.1611328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 100.1201171875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0849609375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721038808307,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 96.34765625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 89.0546875,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721049704402,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 106.326171875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 99.859375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0888671875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721060368322,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 95.4189453125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 92.498046875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.08984375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721071008627,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.4296875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 106.26953125,
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
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721081664142,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.111328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 113.095703125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1044921875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721093213342,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 92.09765625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 95.001953125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1005859375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "e7d3b56c65469bba5cde2d669fbf1ead5927bd5b",
          "message": "Correcting tet",
          "timestamp": "2024-07-13T07:34:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e7d3b56c65469bba5cde2d669fbf1ead5927bd5b"
        },
        "date": 1721103541061,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 89.685546875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 97.5224609375,
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
          "id": "672ae586c12ba9bd7d06013d616e59fd6a541375",
          "message": "Update benchmark.yml",
          "timestamp": "2024-07-16T10:59:23+05:30",
          "tree_id": "f2b7f39d32857ca6646f89cdb8d428771c61150b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/672ae586c12ba9bd7d06013d616e59fd6a541375"
        },
        "date": 1721117398326,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 97.6171875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 109.541015625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.0947265625,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}