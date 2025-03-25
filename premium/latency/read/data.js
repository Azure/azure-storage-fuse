window.BENCHMARK_DATA = {
  "lastUpdate": 1742895237012,
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
          "id": "0fb27e40864763f2c379134fb6e65bab9ebe4cd3",
          "message": "Adding custom component feature (#1558)\n\n* adding custom component feature",
          "timestamp": "2024-11-05T13:10:41+05:30",
          "tree_id": "8ea42764d92489abc1d9e99857af7505ea29622e",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0fb27e40864763f2c379134fb6e65bab9ebe4cd3"
        },
        "date": 1730797878314,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.101914756796,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 68.37438369970299,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.10473971156133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19119107258633336,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10090986492666666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.80441121567999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17317446937633332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1077692186903332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.825515817122,
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
          "id": "696ef5542657b6f118c369abe29b62f392408b3e",
          "message": "Optimize Rename function logic to reduce the number of REST API calls. (#1459)\n\n* remove extra AREST API call in renameFile",
          "timestamp": "2024-11-05T22:04:45+05:30",
          "tree_id": "6a8a94643292f56ac0470aa8c5a20c27d9cb3669",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/696ef5542657b6f118c369abe29b62f392408b3e"
        },
        "date": 1730825701081,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09271295322533334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.32920587158999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09170113699266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18257299912166666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11233773875533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.03390004837334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17858195593899998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.139232708547,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.81780134768066,
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
          "id": "44416e76f2875797446296989a8d9384c624b6f5",
          "message": "Add Checks for temp path in Block Cache while mouting (#1545)\n\n* Add Checks for temp path in Block Cache while mouting",
          "timestamp": "2024-11-05T22:55:42+05:30",
          "tree_id": "ab8d0297ad6b7338eaa3cd87841d4cf01a6a590f",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/44416e76f2875797446296989a8d9384c624b6f5"
        },
        "date": 1730830735784,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10852043613033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.331971535608,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08447434989833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19849750358533336,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.102253688824,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.65886340428234,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.190643499977,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0849695627043332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.82099461822433,
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
          "id": "e53af0f71d8cefc5c268e2a10e1b23498bb2cea6",
          "message": "Support back for object id using azidentity (#1557)\n\n* Support back for object id using azidentity",
          "timestamp": "2024-11-05T22:57:09+05:30",
          "tree_id": "ac60fbf9f0a9576062aa3e65c64bab249ca60239",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e53af0f71d8cefc5c268e2a10e1b23498bb2cea6"
        },
        "date": 1730832158670,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10106734921800002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.79395608987033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07944272761166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.188931722701,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10709818242000001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.776361104098,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1765171566246667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0808460767476669,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.60152028434267,
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
          "id": "e53af0f71d8cefc5c268e2a10e1b23498bb2cea6",
          "message": "Support back for object id using azidentity (#1557)\n\n* Support back for object id using azidentity",
          "timestamp": "2024-11-05T17:27:09Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e53af0f71d8cefc5c268e2a10e1b23498bb2cea6"
        },
        "date": 1731212561064,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09130854667733335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.719099738867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09897662641966666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.162733668651,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09456125576733332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 67.796952344307,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16748891230666665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0825031623740002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.20355656195899,
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
          "id": "6ba5aafb2ef626909574a88c0f540b7084648461",
          "message": "Generate default config  (#1535)\n\n* added command to generate default config",
          "timestamp": "2024-11-11T11:52:03+05:30",
          "tree_id": "2663f0864cc1bacc5f1160ca3de69c557bbad7e1",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/6ba5aafb2ef626909574a88c0f540b7084648461"
        },
        "date": 1731307268555,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09332750831666665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.209417217858,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08987153002800001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18220566765933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09685229884533332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.57889240438567,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17022081701466665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0159524976,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.941933113166,
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
          "id": "829fac9a499893abd9d39e070b0a4c468ed22fca",
          "message": "Updating changelog",
          "timestamp": "2024-11-10T22:26:10-08:00",
          "tree_id": "b408000bbf14c293a175be7ab64ac3e79a77f4d5",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/829fac9a499893abd9d39e070b0a4c468ed22fca"
        },
        "date": 1731308672678,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09438926897066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.12105441268834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07980257803733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19883994923333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10247087674066668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 65.55715423723433,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16478933683066668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1253898075753332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.43423408376499,
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
          "id": "7c3bde15683c301b1a249564e6fe167ce2dad412",
          "message": "Updating version",
          "timestamp": "2024-11-10T22:29:29-08:00",
          "tree_id": "bb70ddced8d4b435c34a7d660f35f5b52d6d264a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7c3bde15683c301b1a249564e6fe167ce2dad412"
        },
        "date": 1731310191288,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09610172111033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.88643128185467,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08192314305633333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18219044619966665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11109259518766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 66.984074676533,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18967384036233334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0857232772613332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.99645899603934,
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
          "id": "3f1ba7a4372644ea538fab38531951c603dfbb4b",
          "message": "Updating changelog",
          "timestamp": "2024-11-10T22:57:27-08:00",
          "tree_id": "0c1103638e0061e3792be32bda55780a932a23b4",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3f1ba7a4372644ea538fab38531951c603dfbb4b"
        },
        "date": 1731311602800,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.084076712216,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.158749854674,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06874953136066665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17581199499666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11205949576633333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.30820146431499,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.174601851831,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1018555533236667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.46743235869666,
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
          "id": "e495570d47ab842ad4d01d356007da266d498828",
          "message": "Correcting code coverage ignore list",
          "timestamp": "2024-11-11T01:00:43-08:00",
          "tree_id": "7ff0cf2efcc7c4a408390db2782c6237cb48fbc3",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e495570d47ab842ad4d01d356007da266d498828"
        },
        "date": 1731316799866,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.097969210844,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.62872873306033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06243990083366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.13695183602333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09851817632800002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 67.56409691520767,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17169706099466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0645933465716668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.31039026642334,
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
          "id": "b26436c81c6abc7d1b9efe6bc38844366d874484",
          "message": "Delete empty directories from local cache (#1524)",
          "timestamp": "2024-11-12T14:37:57+05:30",
          "tree_id": "d7553d6ad2d18fdcb9ecc008e0e7e0e4f07827f7",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/b26436c81c6abc7d1b9efe6bc38844366d874484"
        },
        "date": 1731403637989,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09748689341166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.600102615548,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09684114899033332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17522360734633335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.100540596875,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.44356025671767,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17250000668566667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1265248711636666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.60090759448566,
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
          "id": "e5f6bd7fc2d18b4d6988c741bf08f07d3d10549c",
          "message": "Entry cache component (#1515)\n\n* Adding listing caching option",
          "timestamp": "2024-11-12T15:08:14+05:30",
          "tree_id": "deeabeb069c1a1ae6f177b9c72c2d3ffb206f5f3",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e5f6bd7fc2d18b4d6988c741bf08f07d3d10549c"
        },
        "date": 1731405592933,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09768604479366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.82976493891933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.104338615082,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14598291423566667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10383086077233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.97017532289034,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17343286676033332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0805126819170001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.19685439206033,
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
          "id": "1c99ecdde04702063a06bfd09257184452028800",
          "message": "Remove deprecated methods (#1563)",
          "timestamp": "2024-11-13T12:59:31+05:30",
          "tree_id": "61d9c961df2fa210a68a17564e5fae199446e401",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1c99ecdde04702063a06bfd09257184452028800"
        },
        "date": 1731484103417,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08275620567266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 68.593041471133,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07439449694033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18569241779766665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10211177579300001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.22350568115633,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.171716979002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0712666173693333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.805907967169,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Sourav Gupta",
            "username": "souravgupta-msft",
            "email": "98318303+souravgupta-msft@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "9d552227a816fc6dd1bfe02c3f610ddcd1acee1e",
          "message": "fix (#1567)",
          "timestamp": "2024-11-15T11:27:37Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9d552227a816fc6dd1bfe02c3f610ddcd1acee1e"
        },
        "date": 1731817404153,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09196635191933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.55729257708134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09181503063633334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.20513107583833334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11122476142,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.83241705782832,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16384193740233333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.012117364129,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.28523581979199,
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
          "id": "26b576132e6abcdf2362a35948808b8e0192420d",
          "message": "Rocky UT fails due to insufficient memory (#1570)\n\n* Fixing UT for Rocky env",
          "timestamp": "2024-11-18T15:29:08+05:30",
          "tree_id": "99b522042e9bcb9c46f3d240ee0ab1265ed82420",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/26b576132e6abcdf2362a35948808b8e0192420d"
        },
        "date": 1731925102440,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09080091034033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.11042799045167,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07507278797966667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.149687364866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11058975826900001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.12919983689933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17143023408433336,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.109211648056,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.469497428261,
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
          "id": "cbd7b10085d53e985f1a706fdbff1f7a906753eb",
          "message": "truncate logic correction in filecache (#1569)\n\n* Fix will prevent the truncate in file cache to upload the entire file. This is causing the network error when the file is large.\r\nInstead it passes the call to the next component.",
          "timestamp": "2024-11-18T20:13:40+05:30",
          "tree_id": "d4d87da93630b37a9334068f28241ee5ef01b818",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/cbd7b10085d53e985f1a706fdbff1f7a906753eb"
        },
        "date": 1731942217200,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09409971914033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.55442871936967,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07745308324366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18886405252500002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10400257051133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.010447098705,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.180717284284,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9878659705233334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.00853042084667,
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
          "id": "1ce5915528db77980cae98721e28e37d1455e0f2",
          "message": "Adding Debian 12 (#1572)",
          "timestamp": "2024-11-20T13:30:48+05:30",
          "tree_id": "9f10e0b7bb79f95370f21e432f011631361462c9",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1ce5915528db77980cae98721e28e37d1455e0f2"
        },
        "date": 1732090827520,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09147377566733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.35001017618235,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.089573068313,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.162893609954,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10190286337933334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.05710282524466,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17317263107233336,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9898311960780001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.577127355058,
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
          "id": "0ab1a64ca7c495e9c8db3c26b8d1455cc2702f2a",
          "message": "Move version checks from public container to static website (#1550)\n\n* Adding support for static website for version check",
          "timestamp": "2024-11-22T09:54:39+05:30",
          "tree_id": "050bff422bfad4501d83b4ed2dccc492384c18ad",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0ab1a64ca7c495e9c8db3c26b8d1455cc2702f2a"
        },
        "date": 1732250688789,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08407003799066666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.656493714518,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08711727141566666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18083290897766666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.100895300754,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.50887893999901,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17812341811133334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1203693024343333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.266753834387,
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
          "id": "0ab1a64ca7c495e9c8db3c26b8d1455cc2702f2a",
          "message": "Move version checks from public container to static website (#1550)\n\n* Adding support for static website for version check",
          "timestamp": "2024-11-22T04:24:39Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0ab1a64ca7c495e9c8db3c26b8d1455cc2702f2a"
        },
        "date": 1732422174493,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09067999588866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.06592456407533,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07715699040666667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16557370210233333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09683581609866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.08702962849033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.189192982262,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1146198681473336,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.63393488149067,
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
          "id": "b07781c04ce6f807fcb8622188ddec21e7765bc5",
          "message": "Preserve ACL/Permissions while uploading file over datalake (#1571)\n\n* Add code to reset the ACLs post file upload in adls",
          "timestamp": "2024-11-25T11:22:07+05:30",
          "tree_id": "c58ac7ea6442e09dbae477bc2c40bca751970885",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/b07781c04ce6f807fcb8622188ddec21e7765bc5"
        },
        "date": 1732515063542,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08768201125599999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 68.60836463883366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08909029743533335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18626808389866664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09561550345533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.55772116931733,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1744371113823333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0957049347093335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.36613445476233,
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
          "id": "d84fe700db68ee883f4ab815fd3e1563627a6e2d",
          "message": "Update CHANGELOG.md",
          "timestamp": "2024-11-25T15:45:44+05:30",
          "tree_id": "71bec4d0e7eb310137b0824678f09da1319bc729",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d84fe700db68ee883f4ab815fd3e1563627a6e2d"
        },
        "date": 1732530936153,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09669534135466668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.33328867559901,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08960549241733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17634566859566667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.094573367516,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.08685871664166,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.15949354960499998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1007182375520002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.87250239683767,
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
          "id": "4a57c1b73fe7c0fd587bd9c6a4839f5ad8d0c257",
          "message": "Readme updates for 2.4.0 release (#1583)\n\n* Readme Updated",
          "timestamp": "2024-12-03T10:23:35+05:30",
          "tree_id": "15cb23d5daa5b19f33b6a1da6c610212d2ec8fb2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4a57c1b73fe7c0fd587bd9c6a4839f5ad8d0c257"
        },
        "date": 1733202789335,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09073916700066666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 68.98713725619133,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09052446818466665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.163249987347,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.092615988967,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.02903939868366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16367601770166665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0922710073820001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.17234762271433,
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
          "id": "4a57c1b73fe7c0fd587bd9c6a4839f5ad8d0c257",
          "message": "Readme updates for 2.4.0 release (#1583)\n\n* Readme Updated",
          "timestamp": "2024-12-03T04:53:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4a57c1b73fe7c0fd587bd9c6a4839f5ad8d0c257"
        },
        "date": 1733631812716,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09864033075866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.18930553689367,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08748076147133334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18843495962866666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11001828329866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.87075334778434,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17955576589733335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.078940397392,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.92926918206534,
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
          "id": "de42375d2d71b883acb5a570751fd74d48b28795",
          "message": "Update trivy.yaml",
          "timestamp": "2024-12-10T18:45:04+05:30",
          "tree_id": "8abdb5477018002a44f1e5999ab8e71b2207b2e4",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/de42375d2d71b883acb5a570751fd74d48b28795"
        },
        "date": 1733837703641,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10113697575966667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.24470492022233,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.076446418579,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17026327293833332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10783021261166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 67.64224965295135,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1875295824886667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0733365968196666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.35190682852266,
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
          "id": "de42375d2d71b883acb5a570751fd74d48b28795",
          "message": "Update trivy.yaml",
          "timestamp": "2024-12-10T13:15:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/de42375d2d71b883acb5a570751fd74d48b28795"
        },
        "date": 1734236554710,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.083620987884,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 65.75923933389366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09763146471366668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.202212686401,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09827315836866667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 65.90026712595402,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18456951105966665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0514122862516668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.80406952696099,
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
          "id": "de42375d2d71b883acb5a570751fd74d48b28795",
          "message": "Update trivy.yaml",
          "timestamp": "2024-12-10T13:15:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/de42375d2d71b883acb5a570751fd74d48b28795"
        },
        "date": 1734841381029,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09086952760533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 63.35322249711601,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.081730456055,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18128338330866667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10622150885366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 66.91283916855133,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18520494293200004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.127948806373,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 70.84083214208567,
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
          "id": "de42375d2d71b883acb5a570751fd74d48b28795",
          "message": "Update trivy.yaml",
          "timestamp": "2024-12-10T13:15:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/de42375d2d71b883acb5a570751fd74d48b28795"
        },
        "date": 1735446143918,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10050027043466668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.08023831458466,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08797898980766668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16494535859566664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.098973195019,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.68517288081733,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18719410778733334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0727469405833332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 78.71395031121101,
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
          "id": "5afb77a942b285cd068f086138579ee95e58aaff",
          "message": "updated year in copyright message (#1601)",
          "timestamp": "2025-01-02T15:53:19+05:30",
          "tree_id": "e19601edf66e6eb4ea74d84c1fe2692263689448",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5afb77a942b285cd068f086138579ee95e58aaff"
        },
        "date": 1735814548334,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09460039433166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.11907258950366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08615538044266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.202278068599,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.102269956082,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.017553642138,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.177991523776,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0321110033660001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.28723383561034,
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
          "id": "444abbde1a46fbc0827f5f076b1f1e1c83b96b05",
          "message": "Update dependencies (#1602)",
          "timestamp": "2025-01-03T11:32:59+05:30",
          "tree_id": "27de0581bbed9fb1511b1eb81db1a41d73b385ed",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/444abbde1a46fbc0827f5f076b1f1e1c83b96b05"
        },
        "date": 1735885314933,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09266597613633332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.56998449421867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07560423128566666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18221045745066666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09479126105766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.00598328183733,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18450028610799998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.074638586629333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.501961041878,
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
          "id": "a6009921187eec51bace47eee33f9394c270ca06",
          "message": "Update trivy.yaml",
          "timestamp": "2025-01-06T05:10:34Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a6009921187eec51bace47eee33f9394c270ca06"
        },
        "date": 1736655786073,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.105461427669,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.092200088521,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07733294906266668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17324715765066667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10600532230466668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.51680277245133,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17800565337666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.11590811919,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.80974283694067,
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
          "id": "32ba9ccff233795b83121f0bbab482b2a651ad14",
          "message": "Update CHANGELOG.md",
          "timestamp": "2025-01-15T10:11:40+05:30",
          "tree_id": "5bc21cd06ba56857fc118dc7da71d3eb9ff13ff8",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/32ba9ccff233795b83121f0bbab482b2a651ad14"
        },
        "date": 1736917254605,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09332250268633331,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.12074375130433,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.091322593236,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.138690086672,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10067473747,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.83162072388633,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18617140129200002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0762390245396667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.05638730186733,
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
          "id": "32ba9ccff233795b83121f0bbab482b2a651ad14",
          "message": "Update CHANGELOG.md",
          "timestamp": "2025-01-15T04:41:40Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/32ba9ccff233795b83121f0bbab482b2a651ad14"
        },
        "date": 1737865407631,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08527833863166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.830434558779,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07983273807,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18230420215866663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10932039468566666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.34832860173867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.181910990945,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1120511133126667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.63183824662,
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
          "id": "be07ce307f2902f8fcaede3b3bfabd0444623b00",
          "message": "Merge pull request #1622 from Azure/syeleti/missingReleaseChanges\n\nBug Fixes for Release 2.4.1\r\nUpdate renameFileOptions in fuse2 handler\r\nUpdate the ETag for dst attribute while renaming the file\r\nSanitize the ETag in datalake before adding it to the attribute\r\nInvalidate the attribute in attribute cache after doing a truncate operation on file. This is sort of quick hack done to avoid the complexity of truncate file in az storage component.\r\nAdd atomic_o_trunc flag in fuse2 options that will allow the fuse library to always pass the O_TRUNC flag on open call. see the comments in the code for more details\r\nSend ETag as a struct field to the workitem, as directly retrieving etag from the handle inside the download method is not safe as it might cause a race. seems like Locking the handle to retrieve Etag inside the download method is also a bad idea, as we don't know the handle is already locked/unlocked.\r\nWhen O_TRUNC flag is passed to the open call in file cache and closed immediately without any writes on the data, the old data inside the file is not getting wiped out from the server. fixed this issue\r\nRemove usage of apt command in pipeline as it states that it doesn't have stable CLI interface to use in the scripts. Use always apt-get to install any packages.",
          "timestamp": "2025-02-11T16:03:58+05:30",
          "tree_id": "4531234fa8dab11462f2d7c01a9d90c0650130c1",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/be07ce307f2902f8fcaede3b3bfabd0444623b00"
        },
        "date": 1739271261176,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09485635948,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.339939579207,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07760381220433334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18764585064800002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11587740403066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.68569150613966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18075039148866667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0136191450186667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.820074720372,
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
          "id": "b98358ada922745120776be8689f5a06b41f0c42",
          "message": "Pipeline Changes (#1629)\n\nFix Error: \r\nE: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 6341 (dpkg)\r\nE: Unable to acquire the dpkg frontend lock (/var/lib/dpkg/lock-frontend), is another process using it?\r\nCleanup the pipeline",
          "timestamp": "2025-02-13T04:12:38Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/b98358ada922745120776be8689f5a06b41f0c42"
        },
        "date": 1739679762154,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09376243436266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.78634113881766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09747531089,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14341154469466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10519337702166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.68763732642499,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.19261870854733334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.086649138233,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.30987420090167,
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
          "id": "9157b189c1ce22ed1cb0b78b1d35221e01a1941c",
          "message": "Update CHANGELOG.md",
          "timestamp": "2025-02-18T15:11:28+05:30",
          "tree_id": "ad4bc9bf23aca0eb4e80123e3b2be82e1a3f555c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9157b189c1ce22ed1cb0b78b1d35221e01a1941c"
        },
        "date": 1739872819351,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.11325758730866665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.11816922159166,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07259587771633334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.154129918352,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10226890272,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.98236577528267,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18272184401466665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1151626195153335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.542006943027,
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
          "id": "7bf2f113fa7f641ed1628d642ef9069ca496e26e",
          "message": "In case of mount and mount all, if the container name has '$' do not treat it as env variable (#1637)",
          "timestamp": "2025-02-22T04:33:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7bf2f113fa7f641ed1628d642ef9069ca496e26e"
        },
        "date": 1740284583976,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.11056013465999999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.24478343679333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09663940297933334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.15967980794933334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10272385950133334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.30808813035732,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18415108573966665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1249882223463334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.733578385741,
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
          "id": "62584ff6a5997b1000b168fab568e6c10341e10c",
          "message": "Add support for client assertion based authentication (#1620)\n\nAdd support for client assertion based authentication",
          "timestamp": "2025-02-24T10:37:29+05:30",
          "tree_id": "8ebae39180df473bc4d37acd3cac56a288a49b62",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/62584ff6a5997b1000b168fab568e6c10341e10c"
        },
        "date": 1740374836756,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09313095773366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.29695604623066,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07565759239033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17068658787566662,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09997674434300001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.75699931852233,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18161137172633332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.133129349954,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.541775152466,
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
          "id": "693b1081626de610c2a5d9ce8f3bca8d43837f58",
          "message": "Return true for the commands that were not important (#1653)",
          "timestamp": "2025-03-07T15:00:42+05:30",
          "tree_id": "403998a4e5dd530f2ce889c13804f78a5c9cecff",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/693b1081626de610c2a5d9ce8f3bca8d43837f58"
        },
        "date": 1741341031953,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10259947251200001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.59363999871566,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.10702887009166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.21111223645433333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.107495780005,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.587493792869,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.167986157151,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0569089875383335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 78.55493670537966,
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
          "id": "693b1081626de610c2a5d9ce8f3bca8d43837f58",
          "message": "Return true for the commands that were not important (#1653)",
          "timestamp": "2025-03-07T09:30:42Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/693b1081626de610c2a5d9ce8f3bca8d43837f58"
        },
        "date": 1741494223712,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10322412456399999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.38683571316767,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.092882715773,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19127208972333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10958417078333334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.54742635572332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17531601984,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.98354817965,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.16207406298533,
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
          "id": "508f4a3537fc71585805fe79955ed4074e1e01e9",
          "message": "Creating test file",
          "timestamp": "2025-03-10T23:22:34-07:00",
          "tree_id": "71217f30167c2d85a614ffd4147721e8f52022d5",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/508f4a3537fc71585805fe79955ed4074e1e01e9"
        },
        "date": 1741675268474,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09119727494666667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.84335893221767,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07784973522866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16485325294666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.100221589258,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.03012835524434,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17486427580566666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9724809593163334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.687078056716,
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
          "id": "032e853b3f17a929d8901d16033af0ca712526e7",
          "message": "Adding code",
          "timestamp": "2025-03-10T23:27:15-07:00",
          "tree_id": "cb64d187a30e26511b38bc7bbe16cf108621f804",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/032e853b3f17a929d8901d16033af0ca712526e7"
        },
        "date": 1741677372973,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10989656574000001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.10591579518366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06206408373333333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14830162115533332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09697966108299999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.36242211067967,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18124345042866666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9850293280979999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.50199293922701,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "032e853b3f17a929d8901d16033af0ca712526e7",
          "message": "Adding code",
          "timestamp": "2025-03-11T06:27:15Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/032e853b3f17a929d8901d16033af0ca712526e7"
        },
        "date": 1742099024268,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10026257900933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.293683271396,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08580536410733332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1907908038056667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10575634126333333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 76.016582741265,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16296603183833333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9225066424606667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 80.11653850295801,
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
          "id": "337ac9fe2895b4daad7672cb2aff5dcf391c9398",
          "message": "Testing workflow dispatch",
          "timestamp": "2025-03-18T02:48:58-07:00",
          "tree_id": "c0efd4d607b37d4c16e323d1ace9c126a8c07b8b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/337ac9fe2895b4daad7672cb2aff5dcf391c9398"
        },
        "date": 1742292957977,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08652640394833333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.35390897008968,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.068202520244,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16794991178833332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09416535044166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.028513874994,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16194889926833334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9191974696166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.61248715022599,
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
          "id": "da9870b5ce5f3d8cd141ceb7fadd943f0b0e44f3",
          "message": "testin dispatch",
          "timestamp": "2025-03-18T02:50:42-07:00",
          "tree_id": "9acc3864e5eda91670b8d3a10aca7846c74d6926",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/da9870b5ce5f3d8cd141ceb7fadd943f0b0e44f3"
        },
        "date": 1742295114980,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10059697886233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.94752227417534,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08848609306033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16722997252400004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10063013303266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.73142251306267,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16658533518233332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9453085537093333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 70.52284256388066,
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
          "id": "c0d539db9a6041ab41b8c4c1b5b4909d101dbdfc",
          "message": "Adding workflow dispatch to benchmark test",
          "timestamp": "2025-03-18T02:53:58-07:00",
          "tree_id": "861c51fceaa0c2f15137b89290e469f56a47c181",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c0d539db9a6041ab41b8c4c1b5b4909d101dbdfc"
        },
        "date": 1742297276127,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10016727150299999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.09086888719968,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.062432547894333335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16474327718333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09629249246433334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.28012955399532,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16434037235,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0092315135196668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.91565126390499,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "c0d539db9a6041ab41b8c4c1b5b4909d101dbdfc",
          "message": "Adding workflow dispatch to benchmark test",
          "timestamp": "2025-03-18T09:53:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c0d539db9a6041ab41b8c4c1b5b4909d101dbdfc"
        },
        "date": 1742299463655,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08348221224166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.96724371818932,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09114928967866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18651207347266666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10550265215366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.91863623485966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1612314165183333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9327598458706666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.24011208608734,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "committer": {
            "name": "vibhansa",
            "username": "vibhansa-msft",
            "email": "vibhansa@microsoft.com"
          },
          "id": "c0d539db9a6041ab41b8c4c1b5b4909d101dbdfc",
          "message": "Adding workflow dispatch to benchmark test",
          "timestamp": "2025-03-18T09:53:58Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/c0d539db9a6041ab41b8c4c1b5b4909d101dbdfc"
        },
        "date": 1742703859784,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.098087631269,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.82156738762666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09924814199566666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1745784843563333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10921681147933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 77.85149182517699,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16230821106766669,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9628607339336668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.36347013589267,
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
          "id": "d9ef7ac7a05a6877696828181121741b03c788d9",
          "message": "Adding fix to refill non last block earlier staged with half data (#1643)\n\nCo-authored-by: root <root@anuD16vm.rf4m4cdxc02utd3nay3k4xiwce.xx.internal.cloudapp.net>\nCo-authored-by: ashruti-msft <137055338+ashruti-msft@users.noreply.github.com>\nCo-authored-by: syeleti-msft <syeleti@microsoft.com>\nCo-authored-by: souravgupta-msft <souravgupta@microsoft.com>",
          "timestamp": "2025-03-25T14:44:46+05:30",
          "tree_id": "cc99c0b9facfe38810260c1633a72daee5385a32",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d9ef7ac7a05a6877696828181121741b03c788d9"
        },
        "date": 1742895236761,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10964209893433334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 84.13316595486134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.078581005426,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18260662154500004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.115053888949,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 77.22592890069133,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.19167115838100002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0627567453316666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.248748514126,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}