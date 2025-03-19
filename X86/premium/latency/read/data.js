window.BENCHMARK_DATA = {
  "lastUpdate": 1742366827991,
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
          "id": "e531bd70068a4d7ecb9c2d1096f6cf78f31eadb7",
          "message": "Updated container name",
          "timestamp": "2025-03-07T03:39:29-08:00",
          "tree_id": "d61a69967fd61b62788c82e601611c72fb11db2a",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/e531bd70068a4d7ecb9c2d1096f6cf78f31eadb7"
        },
        "date": 1741348716346,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10326116982166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.89357459824134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09189988627366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18466226270466668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11346633956633334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.269196885535,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18194483633033331,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0002579831616667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.77476939868033,
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
          "id": "db543abdaf167da96dc1aab0033b0b26c065bf7c",
          "message": "Added step to cleanup block-cache temp path on start",
          "timestamp": "2025-03-07T04:43:37-08:00",
          "tree_id": "8efca96f31bbb941ccd6e7c17a880599f40282f3",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/db543abdaf167da96dc1aab0033b0b26c065bf7c"
        },
        "date": 1741352833376,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10036900636099999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.83276671696866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09246410116899999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1842671540533333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10493883863733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 77.84289271953133,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17757954142466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.053313031905,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 78.102085128567,
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
          "id": "03e72e47d37985e5c28051c0ff17bdc0c7315e74",
          "message": "Correcting code for cache cleanup",
          "timestamp": "2025-03-07T23:48:22-08:00",
          "tree_id": "5b69276c81c0c728ae2dd3889b9743194fdcc990",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/03e72e47d37985e5c28051c0ff17bdc0c7315e74"
        },
        "date": 1741421382400,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09256632006333333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.09666017937467,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07800373947533333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1939877511103333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.108224689303,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 78.67962950878068,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.19209700751766667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9427758945660001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 80.25588179344699,
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
          "id": "79978157a7cb7035566f743a8c86becadf2dec81",
          "message": "Merge branch 'main' into vibhansa/armperftest",
          "timestamp": "2025-03-10T22:32:05+05:30",
          "tree_id": "cbd7d68b0a780722eb7ff9ee8e431fec9495a607",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/79978157a7cb7035566f743a8c86becadf2dec81"
        },
        "date": 1741627311531,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09507882687066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.99108030830433,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07478755338333333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.13817687151033334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10746635076233334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 75.40929193626766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17400574075633335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9422937839356668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.17936015234933,
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
          "id": "3b8c1479c707311289f5ee84bf3770b0956497d9",
          "message": "Merge branch 'vibhansa/armperftest' of https://github.com/Azure/azure-storage-fuse into vibhansa/armperftest",
          "timestamp": "2025-03-10T19:45:26-07:00",
          "tree_id": "36cf3d2ea59d4e24ae14350add991d98e2be0d9c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/3b8c1479c707311289f5ee84bf3770b0956497d9"
        },
        "date": 1741662429991,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09313765812199999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.52079122609634,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.081373236557,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.15877536786066665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09770129087666668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.03829968659933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1762682373283333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8508599356936667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.56505839487467,
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
          "id": "4743391f1eac34ad882c8766eb0ee100a2850101",
          "message": "Merge branch 'main' into vibhansa/armperftest",
          "timestamp": "2025-03-13T15:25:11+05:30",
          "tree_id": "d38731698647b2856b859fa97b173461cbae6803",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4743391f1eac34ad882c8766eb0ee100a2850101"
        },
        "date": 1741860978013,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09125845037733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 79.03824105252166,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07874597501766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19972576139733333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10063462065333333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 77.89847325273334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17837533050333332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.902134427231,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.57357017012266,
            "unit": "milliseconds"
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
          "id": "4987ab98e4f8a27a7df0de21978e5ab610135a4d",
          "message": "Remove disk caching from the bench pipeline",
          "timestamp": "2025-03-19T06:25:20Z",
          "tree_id": "0e4f061068b5dcb8afee11fa25a6dcfe27b0d5ef",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4987ab98e4f8a27a7df0de21978e5ab610135a4d"
        },
        "date": 1742366827745,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10885372428833333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.05071034878567,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.10243410481799999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17353024265666664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09363288948233332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 77.88870476590768,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18187431667966666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.985002074399,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.202789378305,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}