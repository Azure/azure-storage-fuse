window.BENCHMARK_DATA = {
  "lastUpdate": 1725769423886,
  "repoUrl": "https://github.com/Azure/azure-storage-fuse",
  "entries": {
    "Benchmark": [
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
          "id": "a42da599c0d65e83577b4235ef0e581e68bd06b3",
          "message": "Making pipeline consistent with the units (#1461)",
          "timestamp": "2024-09-06T15:37:46+05:30",
          "tree_id": "dec09c4da4dcd8fa7bdf1922c2214e2050ee087e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a42da599c0d65e83577b4235ef0e581e68bd06b3"
        },
        "date": 1725618474576,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09551666248266666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 64.16802134391101,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09539765729,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.176531382577,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11035240137433333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 65.35902132115301,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17661499903400002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0689761892533334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 66.40513736587299,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "syeleti-msft",
            "username": "syeleti-msft",
            "email": "syeleti@microsoft.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "a42da599c0d65e83577b4235ef0e581e68bd06b3",
          "message": "Making pipeline consistent with the units (#1461)",
          "timestamp": "2024-09-06T10:07:46Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a42da599c0d65e83577b4235ef0e581e68bd06b3"
        },
        "date": 1725769423665,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09225155477066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 63.27139935360534,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07919504962333333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18776078810466668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09373517270766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 62.60128556554833,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17385730003933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0638351472333334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 66.19421573943266,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}