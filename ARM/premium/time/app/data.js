window.BENCHMARK_DATA = {
  "lastUpdate": 1744303017581,
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
        "date": 1741438727898,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.9445252418518066,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 3.836538076400757,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.9499990940094,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.608469247817993,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8138916492462158,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.811798095703125,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 37.514626026153564,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 14.37084698677063,
            "unit": "seconds"
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
        "date": 1742384045262,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 0.8020122051239014,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.315343141555786,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.53566288948059,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 16.310912609100342,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.743694543838501,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.515763759613037,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.495232343673706,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.25979495048523,
            "unit": "seconds"
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
        "date": 1744276372221,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.1538622379302979,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.166339874267578,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.74119853973389,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 14.783462524414062,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8571562767028809,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.4552149772644043,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 34.0309739112854,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.53883981704712,
            "unit": "seconds"
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
        "date": 1744303017356,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.1065244674682617,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.118869066238403,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.96237897872925,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.46134352684021,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.6379764080047607,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.4582133293151855,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.693076372146606,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.353879451751709,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}