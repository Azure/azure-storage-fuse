window.BENCHMARK_DATA = {
  "lastUpdate": 1720573316471,
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
        "date": 1719986864926,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32009.84765625,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19216.056315104168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5929.5400390625,
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
        "date": 1720006162836,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32266.106770833332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21439.1533203125,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6075.773111979167,
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
        "date": 1720090186707,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32947.478515625,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19891.483723958332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6336.392903645833,
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
        "date": 1720522082622,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31882.383463541668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20704.285481770832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6109.740885416667,
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
        "date": 1720522562590,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 28771.0751953125,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19411.7607421875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 5082.5107421875,
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
        "date": 1720529324429,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31548.229817708332,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 19775.829427083332,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6128.884440104167,
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
        "date": 1720539930081,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32842.272135416664,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20045.155924479168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6192.868489583333,
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
        "date": 1720550769375,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32590.180013020832,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20118.762044270832,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6050.626953125,
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
        "date": 1720561475033,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 31971.309244791668,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 20055.081705729168,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6455.365234375,
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
        "date": 1720573316173,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "seq_write_112_thread",
            "value": 32834.188802083336,
            "unit": "MiB/s"
          },
          {
            "name": "seq_read_128_thread",
            "value": 21603.7607421875,
            "unit": "MiB/s"
          },
          {
            "name": "rand_read_128_thread",
            "value": 6496.71875,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}