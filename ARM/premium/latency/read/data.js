window.BENCHMARK_DATA = {
  "lastUpdate": 1744299240993,
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
          "id": "03e72e47d37985e5c28051c0ff17bdc0c7315e74",
          "message": "Correcting code for cache cleanup",
          "timestamp": "2025-03-07T23:48:22-08:00",
          "tree_id": "5b69276c81c0c728ae2dd3889b9743194fdcc990",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/03e72e47d37985e5c28051c0ff17bdc0c7315e74"
        },
        "date": 1741432081386,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08212261071666667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 62.559968798558,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08478790776066668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.166485268844,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10162477885633332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 57.636108702455665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.12647266981633334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.5496881590483333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 65.85110394025433,
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
          "id": "79978157a7cb7035566f743a8c86becadf2dec81",
          "message": "Merge branch 'main' into vibhansa/armperftest",
          "timestamp": "2025-03-10T22:32:05+05:30",
          "tree_id": "cbd7d68b0a780722eb7ff9ee8e431fec9495a607",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/79978157a7cb7035566f743a8c86becadf2dec81"
        },
        "date": 1741638532343,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08107360485266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.81798461844501,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.083905085714,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.163526457131,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10754774434933334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 65.076717802131,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.12705520466833334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.5506844198856666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.83257762176333,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "syeleti@microsoft.com",
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft"
          },
          "committer": {
            "email": "syeleti@microsoft.com",
            "name": "Srinivas Yeleti",
            "username": "syeleti-msft"
          },
          "distinct": true,
          "id": "abeffac3531ff852a6240abfd960d263274a1527",
          "message": "remove warning errors that is preventing the run to happen",
          "timestamp": "2025-03-19T07:21:38Z",
          "tree_id": "fb855960d36d4e3f6914668bbc2c3a116cd23e8a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/abeffac3531ff852a6240abfd960d263274a1527"
        },
        "date": 1742380573771,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.07263620308233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.84794145184867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07130115093400001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.150326261777,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09091222291533334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 66.591155328341,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.113672483192,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.5262715429753334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 69.14131904876001,
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
          "id": "643e91f6a0e3b6c677d4f89a36e8e5209e046ec6",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/armperftest",
          "timestamp": "2025-04-09T21:55:32-07:00",
          "tree_id": "44e6262b9b6c585dac173204bc04e4eb084a6e47",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/643e91f6a0e3b6c677d4f89a36e8e5209e046ec6"
        },
        "date": 1744273039767,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.07180844098766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.41982723111568,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07429922692466667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14969932288233334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.08980890388,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.27955844590633,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.11349064378533334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.5452393670956668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.77892147530999,
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
          "id": "ba1d1eab212cda59fa94a4ca0f752f2b093ed7c5",
          "message": "Restore mount option'",
          "timestamp": "2025-04-10T04:02:03-07:00",
          "tree_id": "476165a49a1855d8214edc6bb7530574e73094a2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ba1d1eab212cda59fa94a4ca0f752f2b093ed7c5"
        },
        "date": 1744299240759,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.07250988044766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 66.57033855746833,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.075607222181,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.15210609281333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.08615263932966666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.56500457462266,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.11414831573366667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.6431791425273333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 69.21807945922633,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}