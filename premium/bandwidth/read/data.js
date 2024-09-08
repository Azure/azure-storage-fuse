window.BENCHMARK_DATA = {
  "lastUpdate": 1725769422725,
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
        "date": 1725618472628,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2395.6611328125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.8958333333333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2276.9762369791665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1334.5188802083333,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2260.6975911458335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 3.8343098958333335,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4819.560872395833,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3641.3932291666665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 15.029947916666666,
            "unit": "MiB/s"
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
        "date": 1725769421528,
        "tool": "customBiggerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2450.55078125,
            "unit": "MiB/s"
          },
          {
            "name": "random_read",
            "value": 3.9521484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_small_file",
            "value": 2687.4876302083335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_small_file",
            "value": 1238.330078125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2668.5992838541665,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_direct_io",
            "value": 4.0205078125,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 4744.21484375,
            "unit": "MiB/s"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 3626.2952473958335,
            "unit": "MiB/s"
          },
          {
            "name": "random_read_4_threads",
            "value": 15.094075520833334,
            "unit": "MiB/s"
          }
        ]
      }
    ]
  }
}