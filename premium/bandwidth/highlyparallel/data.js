window.BENCHMARK_DATA = {
  "lastUpdate": 1725370415802,
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
        "date": 1720918637874,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31693.913411458332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19979.338216145832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6564.273763020833,
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
        "date": 1720928730558,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31838.309244791668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18551.3544921875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6399.907877604167,
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
        "date": 1720939501498,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32519.270182291668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20200.524739583332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6492.708984375,
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
        "date": 1720950228582,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32187.531575520832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19867.035807291668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6321.3173828125,
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
        "date": 1720961589069,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32982.926432291664,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20724.3447265625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6470.992838541667,
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
        "date": 1720972091979,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31313.09765625,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19735.739583333332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6452.5126953125,
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
        "date": 1720982764234,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32483.611979166668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20067.291666666668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6506.210611979167,
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
        "date": 1720993397616,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32446.050455729168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20805.012369791668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6441.227213541667,
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
        "date": 1721004961354,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31270.38671875,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20232.388346354168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6293.597981770833,
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
        "date": 1721015074804,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31444.776041666668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20257.799479166668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6395.867513020833,
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
        "date": 1721025899780,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31997.011393229168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20484.747395833332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6430.959309895833,
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
        "date": 1721036619679,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32099.313151041668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20075.884765625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6503.2451171875,
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
        "date": 1721047564365,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32678.769856770832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19060.40234375,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6285.268880208333,
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
        "date": 1721058325204,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32038.760416666668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 22088.519856770832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6258.286458333333,
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
        "date": 1721069063859,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31981.124674479168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21290.904947916668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6717.831380208333,
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
        "date": 1721079767014,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32554.643880208332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19354.560221354168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6562.175130208333,
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
        "date": 1721091300366,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31613.708333333332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20636.020833333332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6183.759440104167,
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
        "date": 1721101432843,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31701.668294270832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20716.7041015625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6556.80859375,
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
        "date": 1721110197172,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31606.818033854168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20301.177734375,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6373.096028645833,
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
          "id": "3d46ca9e8a5a564d42ae5aee560bd1dd28c2da66",
          "message": "Cleanup stale mount in remount case (#1453)\n\n* Cleanup in case of mount failure for broken blobfuse mounts",
          "timestamp": "2024-07-17T08:16:58+05:30",
          "tree_id": "29a66a5a93d78532c57947a3754fd31723bee6a9",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3d46ca9e8a5a564d42ae5aee560bd1dd28c2da66"
        },
        "date": 1721186800676,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31934.7607421875,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18221.2890625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6568.094401041667,
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
          "id": "655341f713f53c88772c3b295f30ce124fb89070",
          "message": "Update README.md",
          "timestamp": "2024-07-17T11:01:12+05:30",
          "tree_id": "6b8b43df3616294214f056d61a78e55ac40d4caf",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/655341f713f53c88772c3b295f30ce124fb89070"
        },
        "date": 1721208838266,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32346.566731770832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 17704.091796875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6441.501953125,
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
          "id": "655341f713f53c88772c3b295f30ce124fb89070",
          "message": "Update README.md",
          "timestamp": "2024-07-17T05:31:12Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/655341f713f53c88772c3b295f30ce124fb89070"
        },
        "date": 1721536966485,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31666.0341796875,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20023.204752604168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6098.861328125,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "98318303+souravgupta-msft@users.noreply.github.com",
            "name": "Sourav Gupta",
            "username": "souravgupta-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "73ff84985ac6e6008be4710a4df7c09b405f6d45",
          "message": "Updating TSG for proxy environment (#1464)\n\n* adding tsg for proxy env",
          "timestamp": "2024-07-23T11:09:28+05:30",
          "tree_id": "0fb8febcadbaa4a76e84fa495dcb1d52b5bdee54",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/73ff84985ac6e6008be4710a4df7c09b405f6d45"
        },
        "date": 1721715572268,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31976.146809895832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18399.6416015625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6401.495768229167,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "98318303+souravgupta-msft@users.noreply.github.com",
            "name": "Sourav Gupta",
            "username": "souravgupta-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f2e14e63aedc25674fb8d8a368d93b85e8cf957a",
          "message": "Reset block when released (#1467)",
          "timestamp": "2024-07-24T13:05:43+05:30",
          "tree_id": "52c713d80b62df1e08c91b4068aeefdc8e362ed1",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f2e14e63aedc25674fb8d8a368d93b85e8cf957a"
        },
        "date": 1721809006269,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31194.920572916668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19769.9140625,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 4986.154622395833,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "33023344+tjvishnu@users.noreply.github.com",
            "name": "Vishnu Charan TJ",
            "username": "tjvishnu"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "487ed2288038ddadf79e7311c93c57577ee6ede1",
          "message": "Update README.md (#1477)\n\n* Update README.md",
          "timestamp": "2024-07-27T10:23:35+05:30",
          "tree_id": "12a9104a51fb4f9f2477c6dc3e3c1e73637ace73",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/487ed2288038ddadf79e7311c93c57577ee6ede1"
        },
        "date": 1722058432797,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31988.4189453125,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21261.423502604168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5093.021484375,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Vishnu Charan TJ",
            "username": "tjvishnu",
            "email": "33023344+tjvishnu@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "487ed2288038ddadf79e7311c93c57577ee6ede1",
          "message": "Update README.md (#1477)\n\n* Update README.md",
          "timestamp": "2024-07-27T04:53:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/487ed2288038ddadf79e7311c93c57577ee6ede1"
        },
        "date": 1722141820585,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31525.224283854168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19321.347005208332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5009.066080729167,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "33023344+tjvishnu@users.noreply.github.com",
            "name": "Vishnu Charan TJ",
            "username": "tjvishnu"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "13f5d24be3d63a1ee25fa73953bde300dea7d627",
          "message": "Update README.md (#1480)\n\nAdded link to Known issues",
          "timestamp": "2024-07-28T11:12:31+05:30",
          "tree_id": "dcae62529025eb596da68a37f7e806135edf120f",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/13f5d24be3d63a1ee25fa73953bde300dea7d627"
        },
        "date": 1722152489794,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32037.7119140625,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20196.269205729168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 4779.417643229167,
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
          "id": "db7dec95e4a3a801f94ba8ad8d655b9cfd082886",
          "message": "Blocker and Vulnerability update (#1473)\n\n* Make vulnerability scan a common code",
          "timestamp": "2024-07-29T15:13:48+05:30",
          "tree_id": "8b74e09bd10c1d2c2eaabeba456ecf17545f6c15",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/db7dec95e4a3a801f94ba8ad8d655b9cfd082886"
        },
        "date": 1722248611236,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31857.898763020832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20358.388671875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5193.518880208333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "syeleti@microsoft.com",
            "name": "syeleti-msft",
            "username": "syeleti-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "c0afb6025c561c0d68ae791376d1dba96619612d",
          "message": "Block cache openFile if condition bug (#1472)\n\n* Correct the Condition check that prevents last block to be size greater that block size",
          "timestamp": "2024-07-30T10:56:33+05:30",
          "tree_id": "5c33270c72fa6f8d2a5a2a1521596c718099b557",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c0afb6025c561c0d68ae791376d1dba96619612d"
        },
        "date": 1722319641725,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32861.047526041664,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20545.992513020832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5299.569986979167,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "98318303+souravgupta-msft@users.noreply.github.com",
            "name": "Sourav Gupta",
            "username": "souravgupta-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "ee19eff0729a65b10aa53f8231803d53a0f13e91",
          "message": "Block cache random write in sparse files (#1475)",
          "timestamp": "2024-07-30T16:27:20+05:30",
          "tree_id": "758c81fc793eb2aa990f4990d5dd13c599609dd7",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ee19eff0729a65b10aa53f8231803d53a0f13e91"
        },
        "date": 1722339412954,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31547.9794921875,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19916.5595703125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 4759.554361979167,
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
          "id": "da781335200793f51bbd626f5dfd553009f91cd7",
          "message": "Block Cache Read correction  (#1483)\n\nFixed: We copied the entire block regardless of whether it was fully used, leading to copying over garbage data.\r\nFixed: Error in read when disk cache was enabled",
          "timestamp": "2024-07-31T11:00:28+05:30",
          "tree_id": "504b3aca1f21642ba9f35299b2e1474afdb4fc8e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/da781335200793f51bbd626f5dfd553009f91cd7"
        },
        "date": 1722406209575,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31778.071614583332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19570.2861328125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5155.162434895833,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "98318303+souravgupta-msft@users.noreply.github.com",
            "name": "Sourav Gupta",
            "username": "souravgupta-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "0aa064f0666ab135316608a6973930a514966710",
          "message": "Data integrity issues in block cache (#1508)\n\n* Data Integrity fixes",
          "timestamp": "2024-08-22T18:18:42+05:30",
          "tree_id": "ebb743dc69a1f68a468e060db098ebca06806328",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0aa064f0666ab135316608a6973930a514966710"
        },
        "date": 1724333275822,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 24276.203776041668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19350.194661458332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5040.099934895833,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Sourav Gupta",
            "username": "souravgupta-msft",
            "email": "98318303+souravgupta-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "0aa064f0666ab135316608a6973930a514966710",
          "message": "Data integrity issues in block cache (#1508)\n\n* Data Integrity fixes",
          "timestamp": "2024-08-22T12:48:42Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0aa064f0666ab135316608a6973930a514966710"
        },
        "date": 1724561017132,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 24091.222005208332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 18917.768229166668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5178.8544921875,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "98318303+souravgupta-msft@users.noreply.github.com",
            "name": "Sourav Gupta",
            "username": "souravgupta-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "d06331738096999b358a430fb339e11dcd4d7642",
          "message": "Added block cache limitations (#1511)",
          "timestamp": "2024-08-30T14:53:53+05:30",
          "tree_id": "86cf1bbadf89cc6f72770aaf10a13fed62f9c3ac",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d06331738096999b358a430fb339e11dcd4d7642"
        },
        "date": 1725012271253,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 24374.333658854168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21081.296223958332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 4882.899739583333,
            "unit": "MiB/s"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Sourav Gupta",
            "username": "souravgupta-msft",
            "email": "98318303+souravgupta-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "d06331738096999b358a430fb339e11dcd4d7642",
          "message": "Added block cache limitations (#1511)",
          "timestamp": "2024-08-30T09:23:53Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d06331738096999b358a430fb339e11dcd4d7642"
        },
        "date": 1725165920073,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 24549.619466145832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20362.243815104168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5281.59765625,
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
          "id": "72a988d8c75602ee88954946777dd4042a58d41b",
          "message": "Update CHANGELOG.md",
          "timestamp": "2024-09-03T18:21:44+05:30",
          "tree_id": "a7e791fc9ee02256686000269f891a8e554537aa",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/72a988d8c75602ee88954946777dd4042a58d41b"
        },
        "date": 1725370415462,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 24437.402018229168,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20480.639322916668,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5127.194010416667,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}