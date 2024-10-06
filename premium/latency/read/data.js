window.BENCHMARK_DATA = {
  "lastUpdate": 1728188683417,
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
        "date": 1726222261055,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09404631302566668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 63.33773244178834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09256723702033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18459271746500003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.094682595198,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 65.11317476637034,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16292773307466668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9995305718649998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 68.47895582192166,
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
        "date": 1726263056680,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09244792386766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 63.307541187384,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07759377341033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.15993997339166668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10234835562566667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 65.626929382943,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16895530896733332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9593635365350001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 68.05841143415267,
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
          "id": "820e00f8754bee762743d24d8d1ca79c5b6fd8f8",
          "message": "Fix code coverage tests (#1518)",
          "timestamp": "2024-09-13T21:10:46Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/820e00f8754bee762743d24d8d1ca79c5b6fd8f8"
        },
        "date": 1726374240273,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10184395511666666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 65.58137706665367,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08926559320033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19321832082999998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.109760847443,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 64.48854491537999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1741550598946667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0754172969173332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 66.14716202272434,
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
          "id": "de2f9fdc8ad81619fe931d60b3f6725eb8b9ed42",
          "message": "README update (#1513)",
          "timestamp": "2024-09-17T11:18:15+05:30",
          "tree_id": "37f993037de2984e9b703196038ed156c68ad9a5",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/de2f9fdc8ad81619fe931d60b3f6725eb8b9ed42"
        },
        "date": 1726553330233,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09156949025933335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.48848849870333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.075300360287,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17049820576599997,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.101854588917,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 60.47924073146533,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17115569971666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9569759027616667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 70.806299473154,
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
          "id": "42b3b19b42dbe36e5b37e7b4d81115c6a319b277",
          "message": "Upgrading go version to 1.23.1 (#1521)\n\n* Upgrading go version to 1.23.1:",
          "timestamp": "2024-09-19T15:28:10+05:30",
          "tree_id": "b1f7ff028645e4ea33f7431a9f815fa862d2445c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/42b3b19b42dbe36e5b37e7b4d81115c6a319b277"
        },
        "date": 1726741123375,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10263416184766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 60.00691515072733,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08721231949266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1402478287993333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09873965030866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 62.14723329214966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16871276249333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1119612966586667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 68.55001309537268,
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
          "id": "42b3b19b42dbe36e5b37e7b4d81115c6a319b277",
          "message": "Upgrading go version to 1.23.1 (#1521)\n\n* Upgrading go version to 1.23.1:",
          "timestamp": "2024-09-19T09:58:10Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/42b3b19b42dbe36e5b37e7b4d81115c6a319b277"
        },
        "date": 1726979048994,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10167955486733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 65.05787009937633,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08661033952266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14318759117866667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09558479436133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 63.21138952955033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16976173922433335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9710638363743334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 69.14665641541234,
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
          "id": "42b3b19b42dbe36e5b37e7b4d81115c6a319b277",
          "message": "Upgrading go version to 1.23.1 (#1521)\n\n* Upgrading go version to 1.23.1:",
          "timestamp": "2024-09-19T09:58:10Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/42b3b19b42dbe36e5b37e7b4d81115c6a319b277"
        },
        "date": 1727583842988,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10766507525766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.29616217441766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07463952250233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19804504419333332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09284575676766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.263101156132,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17998223487666665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.910608764449,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.659938940154,
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
          "id": "42b3b19b42dbe36e5b37e7b4d81115c6a319b277",
          "message": "Upgrading go version to 1.23.1 (#1521)\n\n* Upgrading go version to 1.23.1:",
          "timestamp": "2024-09-19T09:58:10Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/42b3b19b42dbe36e5b37e7b4d81115c6a319b277"
        },
        "date": 1728188683189,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10608267729866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.12175310973066,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08108633050666667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18858744269999997,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.099970672042,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.832700785626,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1619957557303333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.919388058831,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.481893108841,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}