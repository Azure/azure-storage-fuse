window.BENCHMARK_DATA = {
  "lastUpdate": 1710437906730,
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
          "id": "c272d8b5b8f90fe9671356efd3e7587be836a870",
          "message": "Correct ioengine",
          "timestamp": "2024-03-14T21:30:19+05:30",
          "tree_id": "97809478e0f900cc621a2be24358a30eb6eef4f7",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c272d8b5b8f90fe9671356efd3e7587be836a870"
        },
        "date": 1710434160601,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8770822192866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.95500704929933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.40159898902566665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.6819019531466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.909903444749,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.41478988569033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.3104025634350003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.2121870510949995,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 72.506766126339,
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
          "id": "b8e905041b73f9a1a1b6e9d8c20adccde8061ff5",
          "message": "Correcting condition",
          "timestamp": "2024-03-14T22:28:41+05:30",
          "tree_id": "761b9036e9e7ecefa4bff1acfa88658816f1f82b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/b8e905041b73f9a1a1b6e9d8c20adccde8061ff5"
        },
        "date": 1710437906084,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8760104535266666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.00654636732833,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.40077807351700007,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.856094331578,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9371153429066668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.80917143922834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_four_threads",
            "value": 2.3311029756493333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_sixteen_threads",
            "value": 4.013385819402333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_four_threads",
            "value": 75.66196319188799,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}