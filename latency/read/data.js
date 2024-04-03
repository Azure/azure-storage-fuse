window.BENCHMARK_DATA = {
  "lastUpdate": 1712139602960,
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
          "id": "5883ec22f417b4d5fd9fd4c6499075aa349ca141",
          "message": "Add sudo to list and delete code",
          "timestamp": "2024-03-22T14:43:56+05:30",
          "tree_id": "9c816d00ef617f69ab0c306d7a0431f1e59f3953",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/5883ec22f417b4d5fd9fd4c6499075aa349ca141"
        },
        "date": 1711101751101,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 2.6211366300439995,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.007853486881,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.29106248481266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.5820595823216667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2.629260067396333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.20648714150134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 3.8965510887243333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 5.489471247097334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 64.269054364864,
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
          "id": "7ac0753f8f1525e7f3e7060acc92944317b91797",
          "message": "Trying to correct list status",
          "timestamp": "2024-03-22T15:58:47+05:30",
          "tree_id": "46607f3062deec168aba3ac182c9b87ab9bd354b",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/7ac0753f8f1525e7f3e7060acc92944317b91797"
        },
        "date": 1711110382989,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8670886372623332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.67510555158866,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.397590830633,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.722283534516,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.8984981400346668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.26982306484534,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2.2959292479756663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4.216588724726667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.969704886452,
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
          "id": "9259d1330426c894a475e96c1c600b40800b3c35",
          "message": "Correct command",
          "timestamp": "2024-03-23T13:06:23+05:30",
          "tree_id": "524a682b3ad8b94921c905da2c29dae2b91a349c",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9259d1330426c894a475e96c1c600b40800b3c35"
        },
        "date": 1711181309206,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8788684439446666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 72.184736536233,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.35823917451366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.7407136273543333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.8939843323113335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.35217628333032,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2.3164033519176668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4.457561958989,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 69.02315324798533,
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
          "id": "22ad862ea4f452937373ac12e63b3d47c90c17d8",
          "message": "Correct list tests",
          "timestamp": "2024-03-26T16:05:36+05:30",
          "tree_id": "0f0fe3892640150bbb1562003cb2bc32f3d336db",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/22ad862ea4f452937373ac12e63b3d47c90c17d8"
        },
        "date": 1711451291583,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8501900436956669,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 67.11308958995835,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.42749963296733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.6555342481973333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 2.0070963657693333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 73.02808318384399,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2.2709522295356668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4.348842141318667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 67.81251731711133,
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
          "id": "40dc943aaeb620f4c1bff6497796f272d091b109",
          "message": "Correct list and del output",
          "timestamp": "2024-03-26T21:55:09+05:30",
          "tree_id": "8be23de9488d8b0c1915d7cd89a304cdeafc44da",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/40dc943aaeb620f4c1bff6497796f272d091b109"
        },
        "date": 1711472281025,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8715957459526666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 69.238598491765,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.3739955597246667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.7804778293753335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9110995184719999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.18722666716,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2.2590010838840002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4.558940112262666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.51300660261666,
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
          "id": "482a82eb5445945508713706c9768c7a442d8c88",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/perftestrunner",
          "timestamp": "2024-04-03T13:58:04+05:30",
          "tree_id": "c4074fe168e90d751e699de98801bf2169e6aac8",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/482a82eb5445945508713706c9768c7a442d8c88"
        },
        "date": 1712134844527,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.878665009174,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 71.62051832207935,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.3170131346423333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.8406203504363333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9165094634936668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 68.10491915366033,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2.305518675383,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4.244372694272667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 72.18330871438634,
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
          "id": "482a82eb5445945508713706c9768c7a442d8c88",
          "message": "Merge remote-tracking branch 'origin/main' into vibhansa/perftestrunner",
          "timestamp": "2024-04-03T13:58:04+05:30",
          "tree_id": "c4074fe168e90d751e699de98801bf2169e6aac8",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/482a82eb5445945508713706c9768c7a442d8c88"
        },
        "date": 1712139602626,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 1.8510081578673334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 70.16226381834933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.341708539546,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.594244130877,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 1.9824578920983331,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 70.741285047286,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 2.2841663419226665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 4.099645218760333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 71.51596040426433,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}