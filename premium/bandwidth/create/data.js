window.BENCHMARK_DATA = {
  "lastUpdate": 1721070827234,
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
        "date": 1719983238187,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 92.1396484375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 89.943359375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1025390625,
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
        "date": 1720013631218,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 87.2978515625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 87.8193359375,
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
        "date": 1720091848109,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 92.3359375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 101.18359375,
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
          "id": "5eabe8a1f5f8325cf880d539c15a503be4d38cb2",
          "message": "Create PerfTest.yml (#1349)\n\n* Create perf test runner to regularly benchmark performance",
          "timestamp": "2024-07-09T15:36:23+05:30",
          "tree_id": "15817378f278eacf9de12eaaa7fdcb7aff2216dc",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5eabe8a1f5f8325cf880d539c15a503be4d38cb2"
        },
        "date": 1720523915463,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 96.1533203125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 92.29296875,
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
        "date": 1720531060452,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 97.990234375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 97.5029296875,
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
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T10:23:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720541696126,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 92.2841796875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 97.712890625,
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
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T10:23:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720552538376,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 89.669921875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 96.39453125,
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
          "id": "98acac361ff7a594d3e2bc18f2eef0d611e055c2",
          "message": "Added min prefetch check (#1446)\n\n* Added check for memsize and prefetch if set by default",
          "timestamp": "2024-07-09T10:23:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/98acac361ff7a594d3e2bc18f2eef0d611e055c2"
        },
        "date": 1720563276116,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 92.361328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 95.748046875,
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
        "date": 1720574717214,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 97.1904296875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 99.8095703125,
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
        "date": 1720581618802,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 105.9541015625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 92.541015625,
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
        "date": 1720595781847,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 97.0869140625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 97.1337890625,
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
        "date": 1720606389289,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 81.0634765625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 94.2060546875,
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
        "date": 1720614449026,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 92.1484375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 77.560546875,
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
        "date": 1720628215032,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 87.6728515625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 85.3310546875,
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
        "date": 1720638795326,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 105.3740234375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 99.6708984375,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720649544749,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.9091796875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 104.919921875,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720661086363,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 103.734375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 115.3134765625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1025390625,
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
        "date": 1720671191944,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.0087890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 101.2861328125,
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
          "id": "f088b803fe387bbb1f5f76caedbe75cf2439b003",
          "message": "Fixed block-cache test (#1454)\n\n* Fix UT for prefetch count",
          "timestamp": "2024-07-10T11:17:29Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f088b803fe387bbb1f5f76caedbe75cf2439b003"
        },
        "date": 1720681859776,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 132.9072265625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 125.296875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1171875,
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
        "date": 1720692567813,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 126.150390625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 133.599609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1181640625,
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
        "date": 1720703448837,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 109.9140625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 115.0341796875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1240234375,
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
        "date": 1720714380112,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.884765625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 104.6455078125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.09765625,
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
        "date": 1720725212182,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 104.9638671875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 100.6640625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.103515625,
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
        "date": 1720735927092,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.822265625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98.2119140625,
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
        "date": 1720747395131,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 110.16796875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 113.751953125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1083984375,
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
        "date": 1720757425693,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 123.74609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 134.2998046875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.115234375,
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
        "date": 1720768179322,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 127.2099609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 121.609375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1201171875,
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
        "date": 1720772168198,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 100.6337890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 96.236328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1064453125,
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
        "date": 1720779136566,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 95.46484375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 103.08203125,
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
        "date": 1720784917556,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 103.2734375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 105.01953125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1064453125,
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
        "date": 1720790143360,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 118.3427734375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 111.2216796875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1103515625,
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
        "date": 1720800796629,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 104.1337890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 99.6015625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.09765625,
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
        "date": 1720811679556,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 100.603515625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98.7060546875,
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
          "id": "7f395591dbea6264df3160b99c37fbaf4baea1dd",
          "message": "ObjectID info updated and simplified base config (#1452)",
          "timestamp": "2024-07-12T10:41:38Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7f395591dbea6264df3160b99c37fbaf4baea1dd"
        },
        "date": 1720822333431,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 105.5185546875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 103.86328125,
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
          "id": "7f395591dbea6264df3160b99c37fbaf4baea1dd",
          "message": "ObjectID info updated and simplified base config (#1452)",
          "timestamp": "2024-07-12T10:41:38Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7f395591dbea6264df3160b99c37fbaf4baea1dd"
        },
        "date": 1720843700819,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 117.8544921875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 132.7490234375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1171875,
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
        "date": 1720854631322,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 128.0732421875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 121.8017578125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1201171875,
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
        "date": 1720860170747,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.4453125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 109.865234375,
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
        "date": 1720865390990,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.78125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 107.583984375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1083984375,
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
        "date": 1720876295176,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 105.6962890625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 107.48046875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1162109375,
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
        "date": 1720887074655,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 103.2626953125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 109.169921875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.103515625,
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
        "date": 1720897912567,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 115.419921875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 111.5693359375,
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
        "date": 1720908707271,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 108.9912109375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 120.7431640625,
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
        "date": 1720920537797,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.595703125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 98.8232421875,
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
        "date": 1720930353207,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 136.5185546875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 134.53515625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1103515625,
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
        "date": 1720941170506,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 122.69921875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 112.6376953125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.10546875,
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
        "date": 1720951862961,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 110.1806640625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 110.4111328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.107421875,
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
        "date": 1720963302240,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 104.5361328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 110.876953125,
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
        "date": 1720973798777,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.21484375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 103.7236328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1025390625,
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
        "date": 1720984443169,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.8623046875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 111.048828125,
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
        "date": 1720995121747,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 107.9091796875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 104.0576171875,
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
        "date": 1721006688150,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.111328125,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 107.81640625,
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
        "date": 1721016737330,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 108.5654296875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 115.2333984375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1064453125,
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
        "date": 1721027594905,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 123.4560546875,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 111.30859375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.103515625,
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
        "date": 1721038278756,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 105.5400390625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 114.4287109375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.1064453125,
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
        "date": 1721049289532,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 116.3193359375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 108.5537109375,
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
        "date": 1721060049999,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 98.318359375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 104.3837890625,
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
        "date": 1721070826885,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "create_1000_files_in_10_threads",
            "value": 101.853515625,
            "unit": "MiB/s"
          },
          {
            "name": "create_1000_files_in_100_threads",
            "value": 106.3037109375,
            "unit": "MiB/s"
          },
          {
            "name": "create_1l_files_in_20_threads",
            "value": 0.09765625,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}