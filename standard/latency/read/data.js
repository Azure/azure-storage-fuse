window.BENCHMARK_DATA = {
  "lastUpdate": 1726268063203,
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
        "date": 1725623464180,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09526664046533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 127.42641558453901,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07801629400366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.191663197889,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09715494205333335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 115.42209294438966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17913055353133334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0509166922393334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 117.305524336462,
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
        "date": 1725774214003,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10156340711033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 120.59260003376399,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07811410984466667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.140569041206,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10847386257966667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 114.551194573408,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17262560554866666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0810543705033335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 117.48596795373999,
            "unit": "milliseconds"
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
          "id": "7e61a85e6cdab572fe2b517fe7045781194807a4",
          "message": "Updating codeowners (#1520)",
          "timestamp": "2024-09-13T15:20:17+05:30",
          "tree_id": "0159fe8edf54260d8f761a1e5227cc318060d19f",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7e61a85e6cdab572fe2b517fe7045781194807a4"
        },
        "date": 1726227109296,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10248392178499999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 121.74820416707867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.078248942649,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16302024856800001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09952617184966667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 118.068152217923,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17695829663333332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0761156393816667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 121.90601979319133,
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
          "id": "820e00f8754bee762743d24d8d1ca79c5b6fd8f8",
          "message": "Fix code coverage tests (#1518)",
          "timestamp": "2024-09-14T02:40:46+05:30",
          "tree_id": "bbfbc2009ef1cd20bb48ffab500f282ec07d640d",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/820e00f8754bee762743d24d8d1ca79c5b6fd8f8"
        },
        "date": 1726268062979,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10280996736833332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 124.95723142977533,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.088225441673,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18067254473266667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10527837280800001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 117.76343266516166,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17246096036233335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.110140105511,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 116.782932430533,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}