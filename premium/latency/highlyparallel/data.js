window.BENCHMARK_DATA = {
  "lastUpdate": 1720755840640,
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
        "date": 1719981543510,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 220.29168538831235,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 438.6152177316447,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1299.1438376580857,
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
          "id": "65a677429517c28a75a8ab6a3051311f44ba96aa",
          "message": "Revert back to parallel runs",
          "timestamp": "2024-07-02T20:58:56-07:00",
          "tree_id": "6f63f586f70486b7a8ef5ccd4a64d7862cb7715c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/65a677429517c28a75a8ab6a3051311f44ba96aa"
        },
        "date": 1720011814446,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 220.2539489459913,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 439.88402635335,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1310.8517119698497,
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
          "id": "65a677429517c28a75a8ab6a3051311f44ba96aa",
          "message": "Revert back to parallel runs",
          "timestamp": "2024-07-02T20:58:56-07:00",
          "tree_id": "6f63f586f70486b7a8ef5ccd4a64d7862cb7715c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/65a677429517c28a75a8ab6a3051311f44ba96aa"
        },
        "date": 1720089953350,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 218.00216127924668,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 423.3773706511967,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1293.1276935541011,
            "unit": "milliseconds"
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
        "date": 1720521991268,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 219.72970233933765,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 409.2252617802257,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1273.8452708151297,
            "unit": "milliseconds"
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
        "date": 1720522553968,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 247.859538004332,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 437.15977418889065,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1521.627552186347,
            "unit": "milliseconds"
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
        "date": 1720529190145,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 217.65962252287832,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 399.246150848989,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1301.4729423684932,
            "unit": "milliseconds"
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
        "date": 1720539877167,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 214.37548973145763,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 443.24932483293065,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1282.6424962562317,
            "unit": "milliseconds"
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
        "date": 1720550726419,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 217.29276217630232,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 411.1273269586566,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1357.7862246649413,
            "unit": "milliseconds"
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
        "date": 1720561443082,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 221.706299024597,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 394.00066442352863,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1239.9334085396215,
            "unit": "milliseconds"
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
        "date": 1720572932369,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 222.97634558670566,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 388.5886666637046,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1289.8449069073465,
            "unit": "milliseconds"
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
        "date": 1720579849215,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 223.393750720969,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 417.53767735442835,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1398.2707189932662,
            "unit": "milliseconds"
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
        "date": 1720593967970,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 222.24916320265402,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 393.9474675084877,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1271.7693503995852,
            "unit": "milliseconds"
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
        "date": 1720604624683,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 224.32757902201266,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 413.0664529423547,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1291.949175828401,
            "unit": "milliseconds"
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
        "date": 1720612588415,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 217.77525713809368,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 412.78142507673266,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1267.5140570061978,
            "unit": "milliseconds"
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
        "date": 1720626264980,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 215.72098906848169,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 420.21596245133304,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1290.4798396048739,
            "unit": "milliseconds"
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
        "date": 1720637101964,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 228.33841048510268,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 443.54294160540195,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1257.7999791551176,
            "unit": "milliseconds"
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
        "date": 1720647805696,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 221.55648929402,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 419.6122662205594,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1278.6226287612392,
            "unit": "milliseconds"
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
        "date": 1720659416639,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 226.48221094276934,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 386.2666550709784,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1296.454366852728,
            "unit": "milliseconds"
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
        "date": 1720669498037,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 224.0482217875937,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 424.852823442334,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1272.0261174600662,
            "unit": "milliseconds"
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
        "date": 1720680293941,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 225.21524435376432,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 420.4328230313013,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1306.2937411747168,
            "unit": "milliseconds"
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
        "date": 1720691010163,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 222.11718475931136,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 422.39427735510935,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1255.616650589475,
            "unit": "milliseconds"
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
        "date": 1720701924998,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 229.36254239462997,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 381.59684860880037,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1288.8822195743378,
            "unit": "milliseconds"
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
        "date": 1720712642506,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 226.789347631664,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 414.58377901332005,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1280.7571325455085,
            "unit": "milliseconds"
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
        "date": 1720723542981,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 231.16103643986932,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 426.7662234127686,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1284.9160613632982,
            "unit": "milliseconds"
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
        "date": 1720734201493,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 221.6983050369247,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 448.07363988539174,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1309.7907621333832,
            "unit": "milliseconds"
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
        "date": 1720745779576,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 224.8165755504277,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 413.9181285447933,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1318.761463262809,
            "unit": "milliseconds"
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
        "date": 1720755840423,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 223.16043254922965,
            "unit": "milliseconds"
          },
          {
            "name": "seq_read_128_thread",
            "value": 411.26991656363134,
            "unit": "milliseconds"
          },
          {
            "name": "rand_read_128_thread",
            "value": 1361.4000106936892,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}