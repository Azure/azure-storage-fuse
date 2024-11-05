window.BENCHMARK_DATA = {
  "lastUpdate": 1730792867321,
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
        "date": 1726378959686,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09372157689766668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 122.91963780354068,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06314481855666666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14372979975900002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.109411392514,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 120.31231706676033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18508109513566665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0809015289956665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 122.743507198204,
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
        "date": 1726558487734,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.104060403493,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 120.61539597042866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08178765858266666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16289470017933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09922726686666666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 120.11971572380001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17154900675566667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1604766045723334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 121.088863803341,
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
        "date": 1726746279913,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10150683911166668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 118.62317768589867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.087949966808,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18865035903799998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09578713934100001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 121.440543970818,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18340789683033334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1054883037873333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 116.776759303878,
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
        "date": 1726983880031,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09750023183,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 128.23981509728966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.0892226733,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.20421091631366664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09500757538799999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 117.292537206372,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17967894859533332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9572826930536666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 122.03536785835466,
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
        "date": 1727588647123,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10355402047466666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 125.795297660558,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09349647076566668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.215359715923,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09738031340733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 123.11395821809901,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18611517544333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9954317204703332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 123.043209734833,
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
        "date": 1728193611307,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09345104162466666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 139.22759603502934,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08813940780566666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19129604817566667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.089828457825,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 129.555051382669,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18296595029666665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.962311930131,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 132.60399049482265,
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
        "date": 1728992009051,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.14633021933933335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 169.99983847012766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07766209406033332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16628050118033333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11667737867399998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 153.26873592712468,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.180539635437,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9809611588116667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 160.78384455391532,
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
        "date": 1728997086098,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09684283887333334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 138.6329677386187,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.087728662709,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1861057687836667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09471978935900001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 133.56919921287434,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.19109769076500002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9935968682593334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 133.09363930201036,
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
        "date": 1730732727114,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10691424692133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 135.35779456005136,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.10006059713666667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18262816186433337,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.095369669545,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 129.94767430773265,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18202973722533333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.131265327747,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 130.819287568856,
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
        "date": 1730792867097,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10314880962400001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 130.743154724123,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07605413422766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17775794788966667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10860416102733332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 127.004478270203,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16920558298266666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1503379766523334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 133.696377683789,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}