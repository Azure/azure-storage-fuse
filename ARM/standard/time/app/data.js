window.BENCHMARK_DATA = {
  "lastUpdate": 1753009428715,
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
        "date": 1741581606449,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.6046786308288574,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 7.068987607955933,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.962143659591675,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 17.734588861465454,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7977554798126221,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.099169015884399,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 37.274746894836426,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 14.67838191986084,
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
        "date": 1742390090171,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.561169147491455,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.878725290298462,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.40088891983032,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 16.07281756401062,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8480195999145508,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.74615740776062,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.055379152297974,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.368434190750122,
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
        "date": 1744283408436,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.4929091930389404,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.485959768295288,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 45.667165756225586,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 17.508986949920654,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.729830265045166,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 2.8371331691741943,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 26.010287761688232,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 8.660654544830322,
            "unit": "seconds"
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
          "id": "0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd",
          "message": "Adding arm based benchmark tests (#1654)\n\nCo-authored-by: Srinivas Yeleti <syeleti@microsoft.com>",
          "timestamp": "2025-04-10T15:43:25Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd"
        },
        "date": 1744361859405,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.3770718574523926,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.594048976898193,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.6491961479187,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.679591417312622,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 1.378108263015747,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.6384198665618896,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.187501668930054,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.028192520141602,
            "unit": "seconds"
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
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1744543242943,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.2622008323669434,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.110636234283447,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.8404221534729,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.216640949249268,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.806145191192627,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.7221200466156006,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.11217451095581,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 12.932368755340576,
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
          "id": "887bdba6cde3bc805787a410ea3fb4520a830392",
          "message": "Updating README for preload",
          "timestamp": "2025-04-13T23:47:01-07:00",
          "tree_id": "07eea2db5ae5f4e8cc51ae7445f1edbfbd81b581",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/887bdba6cde3bc805787a410ea3fb4520a830392"
        },
        "date": 1744637796790,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.5565004348754883,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.377421140670776,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.603787422180176,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 17.341851472854614,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.9414420127868652,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.821089029312134,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 37.56298017501831,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.648493766784668,
            "unit": "seconds"
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
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1745145732654,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 2.4816782474517822,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.342232942581177,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.82114505767822,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 16.388188362121582,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8318073749542236,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.709273099899292,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.65600252151489,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.44099497795105,
            "unit": "seconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "James Fantin-Hardesty",
            "username": "jfantinhardesty",
            "email": "24646452+jfantinhardesty@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "1667ad8b4bebf79badfccb915c351fd3209883a9",
          "message": "Feature: Lazy unmount (#1705)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>",
          "timestamp": "2025-04-26T07:11:48Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1667ad8b4bebf79badfccb915c351fd3209883a9"
        },
        "date": 1745750227011,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.2132399082183838,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.344034910202026,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.36500668525696,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.518099784851074,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8171756267547607,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 4.842648506164551,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.56008553504944,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.251927852630615,
            "unit": "seconds"
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746355617401,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.5728886127471924,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.356314659118652,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.05480647087097,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.368820428848267,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7470710277557373,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.952263355255127,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.56404781341553,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.057832479476929,
            "unit": "seconds"
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746960294536,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 4.579737424850464,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.258725166320801,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 38.40457010269165,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.737042903900146,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7849373817443848,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.6933915615081787,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.52375888824463,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.249288082122803,
            "unit": "seconds"
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1747565196295,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.493678331375122,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.122668027877808,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.17701244354248,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.610770463943481,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 1.2526092529296875,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.7494068145751953,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.49730610847473,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.008918523788452,
            "unit": "seconds"
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
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1748170301468,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.6772301197052002,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.803373575210571,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.556344509124756,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.578595638275146,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.6953966617584229,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.6118648052215576,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.92132616043091,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.113999128341675,
            "unit": "seconds"
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
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1748775635920,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.3189866542816162,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.7756524085998535,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.35752534866333,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.600190162658691,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7852802276611328,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.654008388519287,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 37.66800117492676,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 14.090373516082764,
            "unit": "seconds"
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
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1749379815629,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 2.6640560626983643,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.923706769943237,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.51439332962036,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.805835008621216,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 3.2792792320251465,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.6080915927886963,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.392300844192505,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 12.962930917739868,
            "unit": "seconds"
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
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1749984598528,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 2.1169073581695557,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.831696271896362,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 37.476072788238525,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.030966520309448,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8765997886657715,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.6621615886688232,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.36050486564636,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.220402002334595,
            "unit": "seconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "truealex81",
            "username": "truealex81",
            "email": "45783672+truealex81@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "8b5b9be10c43d6477ae33aa791c04c31537e3902",
          "message": "Update MIGRATION.md (#1837)",
          "timestamp": "2025-06-17T04:53:08Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/8b5b9be10c43d6477ae33aa791c04c31537e3902"
        },
        "date": 1750590044624,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.2057077884674072,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.513526916503906,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.35336089134216,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 17.546740293502808,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8544721603393555,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.598945140838623,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.299795389175415,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.223331928253174,
            "unit": "seconds"
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
          "id": "75a19ccf157f5497d79103bb0f99ddd55b4a5906",
          "message": "Ashruti/script fix (#1842)",
          "timestamp": "2025-06-24T10:29:37Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/75a19ccf157f5497d79103bb0f99ddd55b4a5906"
        },
        "date": 1751199014584,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.5250763893127441,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.520503044128418,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.44748306274414,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 16.862760066986084,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.9858202934265137,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.9710288047790527,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 34.84489607810974,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.317087173461914,
            "unit": "seconds"
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
          "id": "9fd527bf3c9d1c94ddb1f97083248208392e9fdb",
          "message": "fix rhel package installer in nightly pipeline (#1853)",
          "timestamp": "2025-07-04T12:07:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9fd527bf3c9d1c94ddb1f97083248208392e9fdb"
        },
        "date": 1751799225407,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 3.0847878456115723,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 5.682499170303345,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.53626823425293,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.498744249343872,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.7165985107421875,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.6548051834106445,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 33.502466678619385,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.520150184631348,
            "unit": "seconds"
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
          "id": "fa5351e70e3db2fddf8251d9b7d3e8b2b99fe4eb",
          "message": "Update PMC certificate (#1864)",
          "timestamp": "2025-07-09T11:19:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/fa5351e70e3db2fddf8251d9b7d3e8b2b99fe4eb"
        },
        "date": 1752404585554,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.5140736103057861,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 6.898829698562622,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.26989817619324,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 15.532153606414795,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8851003646850586,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.646371841430664,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 32.97529077529907,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.499245882034302,
            "unit": "seconds"
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
          "id": "05bb0853557011f6824b7738d633b063cf404bcc",
          "message": "Provide a mode to just disable kernel cache not the blobfuse cache (#1882)",
          "timestamp": "2025-07-17T13:55:25Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/05bb0853557011f6824b7738d633b063cf404bcc"
        },
        "date": 1753009428438,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "write_1GB",
            "value": 1.7605159282684326,
            "unit": "seconds"
          },
          {
            "name": "write_10GB",
            "value": 4.4557764530181885,
            "unit": "seconds"
          },
          {
            "name": "write_100GB",
            "value": 39.26803779602051,
            "unit": "seconds"
          },
          {
            "name": "write_40GB",
            "value": 16.28773260116577,
            "unit": "seconds"
          },
          {
            "name": "read_1GB",
            "value": 0.8849632740020752,
            "unit": "seconds"
          },
          {
            "name": "read_10GB",
            "value": 3.628613233566284,
            "unit": "seconds"
          },
          {
            "name": "read_100GB",
            "value": 42.87396788597107,
            "unit": "seconds"
          },
          {
            "name": "read_40GB",
            "value": 13.671419858932495,
            "unit": "seconds"
          }
        ]
      }
    ]
  }
}