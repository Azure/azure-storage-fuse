window.BENCHMARK_DATA = {
  "lastUpdate": 1756009824136,
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
        "date": 1742370166210,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10406387765766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 77.400831000442,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07884872601933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18523540416466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09308974903566665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 76.36058324889267,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17708251150466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.974043242563,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.74422645507767,
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
          "id": "643e91f6a0e3b6c677d4f89a36e8e5209e046ec6",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/armperftest",
          "timestamp": "2025-04-09T21:55:32-07:00",
          "tree_id": "44e6262b9b6c585dac173204bc04e4eb084a6e47",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/643e91f6a0e3b6c677d4f89a36e8e5209e046ec6"
        },
        "date": 1744262204459,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09308025050066666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 68.69252118802133,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.082788543464,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19037204095266666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10367410353966666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 76.012311433923,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17313173703166665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9409528567246667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 75.69695782694767,
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
          "id": "a255ef1080a4bee70d172d6b9d86109bc75a69ae",
          "message": "Updating configs",
          "timestamp": "2025-04-10T02:02:50-07:00",
          "tree_id": "eab286874e9c1ba63c0fe44c0be17d45750a7853",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a255ef1080a4bee70d172d6b9d86109bc75a69ae"
        },
        "date": 1744277269447,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10350988535166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 83.55385593153733,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08476480561966666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.07714835578066666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09713605361133333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 77.98494126035634,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.13092953889933334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.47363645241099994,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 80.45452180035868,
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
          "id": "ba1d1eab212cda59fa94a4ca0f752f2b093ed7c5",
          "message": "Restore mount option'",
          "timestamp": "2025-04-10T04:02:03-07:00",
          "tree_id": "476165a49a1855d8214edc6bb7530574e73094a2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ba1d1eab212cda59fa94a4ca0f752f2b093ed7c5"
        },
        "date": 1744286676551,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09983865123666667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 81.258938642188,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.100629255639,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16203028612700002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10039593678266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.44870543296334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17110602902566666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8600774915673334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 82.039316909471,
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
          "id": "0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd",
          "message": "Adding arm based benchmark tests (#1654)\n\nCo-authored-by: Srinivas Yeleti <syeleti@microsoft.com>",
          "timestamp": "2025-04-10T21:13:25+05:30",
          "tree_id": "476165a49a1855d8214edc6bb7530574e73094a2",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd"
        },
        "date": 1744301212258,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10070130144900001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 83.01484385149134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07785978653066666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17473428199166666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.095972899274,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 80.76403753421867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1617139148796667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8737790331793333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 78.46674001474534,
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
          "id": "0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd",
          "message": "Adding arm based benchmark tests (#1654)\n\nCo-authored-by: Srinivas Yeleti <syeleti@microsoft.com>",
          "timestamp": "2025-04-10T15:43:25Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/0cd7596da267f6a7f6ad7cd9126fc9ae1305d3bd"
        },
        "date": 1744339746264,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10685798454499999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.86464557187499,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.073849549196,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.15143442845833333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09658880005566667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 75.082616347807,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1744271851203333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8894351027466666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.024949182194,
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
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1744519972512,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09916061831233332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 76.47987485625266,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08577577013166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17570488766933334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09254208666033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 76.86246646555367,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16156093522333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8714055763793334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.22179285420833,
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
          "id": "887bdba6cde3bc805787a410ea3fb4520a830392",
          "message": "Updating README for preload",
          "timestamp": "2025-04-13T23:47:01-07:00",
          "tree_id": "07eea2db5ae5f4e8cc51ae7445f1edbfbd81b581",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/887bdba6cde3bc805787a410ea3fb4520a830392"
        },
        "date": 1744614655036,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09468105875433334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.65862203905466,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08577976708599999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.168481026358,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10980620914366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 75.07862094233434,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.169646446873,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9465181327463333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 81.00721873717201,
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
          "id": "07c91329ece3d6310f1a56cdead7b10e449fc66f",
          "message": "Preload feature drop to main branch (#1678)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>\nCo-authored-by: souravgupta <souravgupta@microsoft.com>",
          "timestamp": "2025-04-11T09:17:56Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/07c91329ece3d6310f1a56cdead7b10e449fc66f"
        },
        "date": 1745123102389,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09414728226266666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.16403967474434,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07508490048166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.166916610101,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09300492764333333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.84728617932433,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17210437578933332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8890672015486666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 78.06843582470701,
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
          "id": "43f48e9b789a9fc27d2138c4679ef8dc47cd55bf",
          "message": "Updating README for preload (#1685)\n\nCo-authored-by: Sourav Gupta <98318303+souravgupta-msft@users.noreply.github.com>",
          "timestamp": "2025-04-22T12:59:00+05:30",
          "tree_id": "97239a1303fad55d8f7adfa85b21e4ff6c56579d",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/43f48e9b789a9fc27d2138c4679ef8dc47cd55bf"
        },
        "date": 1745308267645,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.3791414690973334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 79.16023044197799,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08855493819366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17508761669233333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.1961480961016667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 80.75958357814834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.255176782125,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4.827176639875334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 81.376244705732,
            "unit": "milliseconds"
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
        "date": 1745727950665,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09879098787366665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.59147882642701,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08874529266033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18297717427033333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09880658980899999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.699593109915,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.205107971677,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9266229431686667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.23316223685732,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746332847472,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09257733517233334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 81.903284919134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07940842532366665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16985373529633332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10609834195266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.29719130047033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.173546298307,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9344220906973333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.26429762310902,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1746937641154,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10160091826166666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.790161841021,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06867575264766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19009024878433334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10756449695300001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 67.985094438059,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.4281181496636666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 2.15739305998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 109.17598161801966,
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
          "id": "4ddfcd4b776650ae5172663c04db2a0fb791cbd6",
          "message": "Fix logs using up all the space of /tmp folder (#1723)",
          "timestamp": "2025-05-03T08:50:17Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4ddfcd4b776650ae5172663c04db2a0fb791cbd6"
        },
        "date": 1747542563875,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.085754412637,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 75.57320882876166,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08321502817066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.187734586899,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10152225375366668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 75.97145453364766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17816232809800003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8785859588576667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.31575648154333,
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
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1748147397291,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09339580742333332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 78.48794068572234,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.081518985838,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18958507597100002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10394086207066666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 69.63510670253066,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16195526846366667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.913655509573,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 77.18135327232034,
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
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1748752675628,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09290425237733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 63.763868685445004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09624625363333333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19556779167333335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09900677885800001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.887307330726,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1843805645826667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9035878500209998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.95098551181367,
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
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1749357195270,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09725503003499998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.25020622858032,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09333292498333334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.168096446851,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09223352403833333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 65.145451114612,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.15976798789733335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8675126730936666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.678333374476,
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
          "id": "5e0ecbfd0e63a70dca732601790538e455816b41",
          "message": "Mariner preview release location fix (#1785)",
          "timestamp": "2025-05-23T09:32:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5e0ecbfd0e63a70dca732601790538e455816b41"
        },
        "date": 1749961924650,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08250033564066667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.35871504608366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08554873973833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16421084934233332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09560238712366669,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 75.25173350193067,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1691482588336667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8989661802513332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 69.83626225942868,
            "unit": "milliseconds"
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
        "date": 1750566784060,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09469669218566668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.18538409435867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07830397702033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.138133819351,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09633693790566666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 71.20635254241533,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.173425409885,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.874862939961,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 68.70604379063201,
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
          "id": "75a19ccf157f5497d79103bb0f99ddd55b4a5906",
          "message": "Ashruti/script fix (#1842)",
          "timestamp": "2025-06-24T10:29:37Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/75a19ccf157f5497d79103bb0f99ddd55b4a5906"
        },
        "date": 1751171780502,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08238300806766666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 77.26148649910333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07323506403833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19246711111366666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.091045560131,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.634308525995,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.15995602623866667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8678915022623332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.90515497860532,
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
          "id": "9fd527bf3c9d1c94ddb1f97083248208392e9fdb",
          "message": "fix rhel package installer in nightly pipeline (#1853)",
          "timestamp": "2025-07-04T12:07:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9fd527bf3c9d1c94ddb1f97083248208392e9fdb"
        },
        "date": 1751776576813,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.089139498922,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 73.03809808324833,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08465077305299999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18673612368766665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09261256590566667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 66.43810332082568,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16937953695400002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8438730105946667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 74.01561840612167,
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
          "id": "fa5351e70e3db2fddf8251d9b7d3e8b2b99fe4eb",
          "message": "Update PMC certificate (#1864)",
          "timestamp": "2025-07-09T11:19:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/fa5351e70e3db2fddf8251d9b7d3e8b2b99fe4eb"
        },
        "date": 1752381621898,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.081256506026,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.18465600112133,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09152116622900001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19872968427299997,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09144764162933333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.60651031393932,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.164209584299,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8556925418903333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.452348237764,
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
          "id": "05bb0853557011f6824b7738d633b063cf404bcc",
          "message": "Provide a mode to just disable kernel cache not the blobfuse cache (#1882)",
          "timestamp": "2025-07-17T13:55:25Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/05bb0853557011f6824b7738d633b063cf404bcc"
        },
        "date": 1752986494286,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08225516151833333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.10487375796566,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06288628838733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.164685375591,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09844031747133335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.27346698046101,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16888268007333332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8751728470393333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 76.32077599781967,
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
          "id": "7776998e3f031e791148482db29f8c40beb53255",
          "message": "Add New stage to the Nightly pipeline (#1889)",
          "timestamp": "2025-07-24T07:31:00Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7776998e3f031e791148482db29f8c40beb53255"
        },
        "date": 1753591299919,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08358372682733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.329946757059,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.073675335838,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1966938731,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09518942822766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 66.995255562082,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16830386458166666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.925781301011,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 69.52184323424699,
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
          "id": "171080b657b4c7271b928597d9d8534c56da06a3",
          "message": "Convert read options struct to pointer in the pipeline (#1901)",
          "timestamp": "2025-08-02T11:39:27Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/171080b657b4c7271b928597d9d8534c56da06a3"
        },
        "date": 1754196292447,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.095641684784,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.31310974899266,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.088189430711,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18732569904166665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09919161978266666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.40415264889268,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17262238145966669,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8875061734699999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.633023953992,
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
          "id": "1a7d91ce786c8012c8afe26308e2c4d05fefd6aa",
          "message": "fix e2e tests failure  (#1913)\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>",
          "timestamp": "2025-08-05T09:59:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a7d91ce786c8012c8afe26308e2c4d05fefd6aa"
        },
        "date": 1754800822433,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08594613069399999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.28957977983232,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07735189532833335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.168578411618,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09238685192566666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 74.58761627317966,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.15603675584433332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8636697015366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.61592510017067,
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
          "id": "1a7d91ce786c8012c8afe26308e2c4d05fefd6aa",
          "message": "fix e2e tests failure  (#1913)\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>",
          "timestamp": "2025-08-05T09:59:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a7d91ce786c8012c8afe26308e2c4d05fefd6aa"
        },
        "date": 1755405291408,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08917804547766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.90107384864068,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07675744421033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.20192575567933332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.106745460455,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 72.294565626368,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17625284206666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.901819756813,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.30006603497766,
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
          "id": "1a7d91ce786c8012c8afe26308e2c4d05fefd6aa",
          "message": "fix e2e tests failure  (#1913)\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>",
          "timestamp": "2025-08-05T09:59:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/1a7d91ce786c8012c8afe26308e2c4d05fefd6aa"
        },
        "date": 1756009823686,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09612820336600002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 74.03625166751965,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.085572615503,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.187768617361,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09168313531100002,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.126735635925,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.153244407883,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.8711627819656665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 73.85067132439634,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}