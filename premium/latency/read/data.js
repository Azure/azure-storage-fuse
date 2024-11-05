window.BENCHMARK_DATA = {
  "lastUpdate": 1730788116088,
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
        "date": 1728793470973,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09666155447866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 77.657108842201,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09897603747066668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17414341176466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09962655594566668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 78.20413325458499,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17739400331400001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9243733012866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.26627564949733,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "abhigupta9910@gmail.com",
            "name": "abhiguptacse",
            "username": "abhiguptacse"
          },
          "committer": {
            "email": "abhigupta9910@gmail.com",
            "name": "abhiguptacse",
            "username": "abhiguptacse"
          },
          "distinct": true,
          "id": "1f78a4b59edf218e8316101454351135faf286db",
          "message": "adding copyright",
          "timestamp": "2024-10-15T08:10:23Z",
          "tree_id": "b062c447ae3ceea988606d4e0d41ddb85a318468",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1f78a4b59edf218e8316101454351135faf286db"
        },
        "date": 1728982175857,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09415529474900002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 80.509251590649,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.10105480662133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.190963331635,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.101735907862,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 78.064086642725,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18990366857066668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9689399390549999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.27586091730034,
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
          "id": "8c6a53e527de17038fe4728d69ffee532c62f360",
          "message": "Reverting custom component patch (#1541)",
          "timestamp": "2024-10-15T14:25:54+05:30",
          "tree_id": "b1f7ff028645e4ea33f7431a9f815fa862d2445c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/8c6a53e527de17038fe4728d69ffee532c62f360"
        },
        "date": 1728986983874,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09912399755500001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.19575261084167,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08903219566300001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18539084014533333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10209432490499999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.961728500131,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.182276400904,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0016936578073334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.304911887204,
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
          "id": "50cdc3b81b7a025a35f154c8367f1827ac75adad",
          "message": "Update CODEOWNERS",
          "timestamp": "2024-10-17T15:23:37+05:30",
          "tree_id": "25ffa0338213439f79e5915238c0e52e506b1b45",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/50cdc3b81b7a025a35f154c8367f1827ac75adad"
        },
        "date": 1729160053081,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.097897314907,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.50464245164301,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.083358706051,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.136740461983,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10706601997133332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.71212817526833,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17439177228566663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1103769214910002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.56624832457233,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "jainakanksha@microsoft.com",
            "name": "jainakanksha-msft",
            "username": "jainakanksha-msft"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "117411a55a653a0e2bc8d541ae666dc8e5e29a5c",
          "message": "Creating a PR template (#1546)",
          "timestamp": "2024-10-17T17:36:28+05:30",
          "tree_id": "a091ab306dddf7918f4d85e04d46a08875db8281",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/117411a55a653a0e2bc8d541ae666dc8e5e29a5c"
        },
        "date": 1729168045877,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.089353906288,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.95040931099966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06526460816533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.20316805614833333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10400474847966668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.90437622223934,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.181560798769,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1017890035113334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.89470036991968,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "jainakanksha-msft",
            "username": "jainakanksha-msft",
            "email": "jainakanksha@microsoft.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "117411a55a653a0e2bc8d541ae666dc8e5e29a5c",
          "message": "Creating a PR template (#1546)",
          "timestamp": "2024-10-17T12:06:28Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/117411a55a653a0e2bc8d541ae666dc8e5e29a5c"
        },
        "date": 1729398233454,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09279768808433335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.34729682374267,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08571905808833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18243032675366666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09979639426466667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.45616303054733,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17097477541933337,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.060490650638,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.95392010562067,
            "unit": "milliseconds"
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
          "id": "4a9d9ab9024a5103258185b5744237c255f63cbd",
          "message": "Merge pull request #1548 from Azure/tjvishnu-patch-3\n\nUpdate README.md",
          "timestamp": "2024-10-23T09:51:14-07:00",
          "tree_id": "e9172ff88945c2b0961534727238c6df7ad044cd",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4a9d9ab9024a5103258185b5744237c255f63cbd"
        },
        "date": 1729703507204,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09492481514233335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.735107843178,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.086506164118,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.21268282414700002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09543643779466666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 75.30713065657267,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18751239783933335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0293893017963334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.76414046334266,
            "unit": "milliseconds"
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
          "id": "4a9d9ab9024a5103258185b5744237c255f63cbd",
          "message": "Merge pull request #1548 from Azure/tjvishnu-patch-3\n\nUpdate README.md",
          "timestamp": "2024-10-23T16:51:14Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4a9d9ab9024a5103258185b5744237c255f63cbd"
        },
        "date": 1730003064261,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10937868727766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.51906353554233,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09665826068033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.180920336037,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09625245734766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.76832597158966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.188880310237,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0372476943643334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.44366712310868,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "31076415+abhiguptacse@users.noreply.github.com",
            "name": "Abhinav Gupta",
            "username": "abhiguptacse"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "5728405152eeb763c374cdafef497c7383fb9483",
          "message": "adding exported package (#1553)",
          "timestamp": "2024-11-04T15:13:16+05:30",
          "tree_id": "3eadcfb2df339d69d9353aa709b1b5065dc107d8",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5728405152eeb763c374cdafef497c7383fb9483"
        },
        "date": 1730714545528,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09115929190966666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 68.76507192864966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.085539006591,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.167225222175,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.093072728469,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 67.83041459042668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16794910113566666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0103875732506666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.66995540208067,
            "unit": "milliseconds"
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
          "id": "ba739b620646363423170970fd7781339adcdca9",
          "message": "Fix ReadPanic (#1533)\n\n* Fix Read Panic",
          "timestamp": "2024-11-04T18:50:08+05:30",
          "tree_id": "b57a82f3443eb38667887cb22ec37da11d54617a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ba739b620646363423170970fd7781339adcdca9"
        },
        "date": 1730727702462,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10121391986033335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.33310267068033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.087082556445,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17612149077466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10394319245933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.79459699587034,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17666222429466663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0861450420546668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.813039127527,
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
          "id": "46a557ce18bc597d06f560f73f4d792c70a72563",
          "message": "Redirecting stream config to BlockCache (#1445)\n\n* Auto convert streaming config to block-cache config",
          "timestamp": "2024-11-05T11:38:27+05:30",
          "tree_id": "73c016b08ff97d4da1a648b55454b4e182c84916",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/46a557ce18bc597d06f560f73f4d792c70a72563"
        },
        "date": 1730788115836,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10596419421566668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.724421356063,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09006004102933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14031432414333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09352175760133334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.67059657364634,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.184188953651,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0969037938876667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.27255768255134,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}