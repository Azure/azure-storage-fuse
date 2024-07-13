window.BENCHMARK_DATA = {
  "lastUpdate": 1720907013667,
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
        "date": 1719981542311,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32816.904947916664,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18754.072265625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6361.877604166667,
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
        "date": 1720011813042,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32679.139973958332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18707.8515625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6314.461263020833,
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
        "date": 1720089952145,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32954.595052083336,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19412.195638020832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6394.370768229167,
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
        "date": 1720521989900,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32759.7099609375,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20080.544921875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6489.706380208333,
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
        "date": 1720522552686,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 29635.356770833332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19400.929361979168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5234.258138020833,
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
        "date": 1720529188392,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 33045.918294270836,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20598.4267578125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6352.820963541667,
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
        "date": 1720539875785,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 33503.5751953125,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18557.741536458332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6445.2880859375,
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
        "date": 1720550724954,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 33111.420572916664,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19993.546549479168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6149.747395833333,
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
        "date": 1720561440491,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32423.123046875,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20860.3330078125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6669.423177083333,
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
        "date": 1720572930966,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32314.341471354168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21196.017252604168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6414.25,
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
        "date": 1720579847872,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32246.1640625,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19708.4833984375,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5949.368489583333,
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
          "id": "5d7a9e4a7bb4ae16ed9b50d878fa817c40b1d284",
          "message": "Update benchmark.yml",
          "timestamp": "2024-07-10T02:11:14Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5d7a9e4a7bb4ae16ed9b50d878fa817c40b1d284"
        },
        "date": 1720593966533,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32400.720052083332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20882.378255208332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6502.463541666667,
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
        "date": 1720604622802,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32107.81640625,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19901.244140625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6397.957356770833,
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
        "date": 1720612587217,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32973.9169921875,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19922.98046875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6517.326171875,
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
        "date": 1720626263367,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 33290.563151041664,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19576.970703125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6406.5107421875,
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
        "date": 1720637100371,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31541.542317708332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18441.58203125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6569.899739583333,
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
        "date": 1720647804448,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32490.989908854168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19589.6572265625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6467.959635416667,
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
        "date": 1720659415244,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31915.371744791668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21282.2763671875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6377.3603515625,
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
        "date": 1720669496563,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32246.707356770832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19347.411783854168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6500.894856770833,
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
        "date": 1720680292547,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31977.028971354168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19583.335286458332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6332.603190104167,
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
        "date": 1720691008579,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32439.532877604168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19484.929036458332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6583.030598958333,
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
        "date": 1720701923337,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31418.621744791668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21567.4033203125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6415.787760416667,
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
        "date": 1720712638331,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31794.111002604168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19826.822916666668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6455.549479166667,
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
        "date": 1720723541654,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31297.074869791668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19231.934244791668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6434.0458984375,
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
        "date": 1720734200030,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32456.37109375,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18319.652018229168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6321.985026041667,
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
        "date": 1720745778270,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32082.853190104168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19863.728841145832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6276.326822916667,
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
        "date": 1720755838925,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32258.2724609375,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20003.501953125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6116.6708984375,
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
        "date": 1720766636402,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32730.518880208332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21087.600911458332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6373.338216145833,
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
        "date": 1720770339386,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 29812.997395833332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21112.258138020832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5361.21875,
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
        "date": 1720777400394,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32632.997395833332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19916.039713541668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6426.429361979167,
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
        "date": 1720783270384,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31867.359049479168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18964.206705729168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6482.804361979167,
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
        "date": 1720788492750,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31141.161458333332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19535.270182291668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6378.897786458333,
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
        "date": 1720799042925,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32506.996744791668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18911.479817708332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6441.3447265625,
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
        "date": 1720809914788,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31777.432942708332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20977.522786458332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6479.844401041667,
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
        "date": 1720820569693,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32673.819986979168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20062.833658854168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6577.302734375,
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
        "date": 1720842129600,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31860.779947916668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19546.148763020832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6393.402018229167,
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
        "date": 1720853082029,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31766.095377604168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20247.0458984375,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6521.2392578125,
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
        "date": 1720858443087,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32143.206380208332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20893.2392578125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6566.502604166667,
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
        "date": 1720863774548,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31696.435221354168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19937.452799479168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6398.819986979167,
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
        "date": 1720874718286,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31562.609049479168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19978.458658854168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6476.553385416667,
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
        "date": 1720885394157,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31813.345703125,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21522.3798828125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6391.119791666667,
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
        "date": 1720896203726,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31609.523111979168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20458.793294270832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6284.2822265625,
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
        "date": 1720907013383,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31880.888671875,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20443.7666015625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6250.3037109375,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}