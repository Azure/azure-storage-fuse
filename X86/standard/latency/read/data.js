window.BENCHMARK_DATA = {
  "lastUpdate": 1750572684428,
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
        "date": 1741426022487,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.096447993442,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 147.8490990167323,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06309361017466668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18452272808466666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11299851516600001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 153.69699595771735,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18121616616766667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0127051680446668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 149.43518789142732,
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
        "date": 1741632393001,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08606705954933334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 142.54389964454967,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.078415004612,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1674058028583333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09791609649599999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 135.534866704028,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.181783134755,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9798878415543334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 139.35775692716498,
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
        "date": 1742375062217,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.11272281248266665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 192.39389834618632,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.089125869681,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.13779662118833333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10360486973766665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 167.724603504908,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16758427275566667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0476406299803331,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 172.37943743776202,
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
        "date": 1744267121739,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10092006355933332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 139.55876092133835,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08862760928866668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.20186013369966668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10991717984499999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 133.20581708836366,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.170561112789,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9903861514826667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 136.38058821861264,
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
        "date": 1744292549256,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09857400274700001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 147.64745995180735,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.088500417399,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.15774435368233333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11082431982133334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 143.48050496280734,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.28007638353133335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0256608150486668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 141.368732319875,
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
        "date": 1744345264010,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09712429587133332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 145.413441199194,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07969280323033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19283043834866664,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10553000498366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 136.545342349021,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17751793370366667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0146682529633333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 138.28897908402934,
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
        "date": 1744525855535,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09705942897233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 134.65727066703698,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.10642609451833333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.17678416613099998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10239804605033333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 128.16287078390832,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16810503520133335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9911863213653334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 132.853200696041,
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
        "date": 1744620550088,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09785525214433334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 136.56768118540467,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08292501390666666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.15339039354566666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.11159254254966666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 126.12664546033834,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.178706197161,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.014808425063,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 127.70416269851368,
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
        "date": 1745128802652,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10503760222399999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 137.59889393548735,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.072092543769,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.137644780379,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09844031634566668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 128.75223761069435,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17357046869966666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.928444835673,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 131.22373161921234,
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
        "date": 1745314250828,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10090993075766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 152.90606977544869,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08809032983866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16602812215799997,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10179309271533334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 138.59821203060002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.16791174038933332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.966722836499,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 140.43991843808033,
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
        "date": 1745733654742,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.17560228141566667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 186.83935707434398,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.106770469016,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18396889840600003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.12369136976200001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 170.54240389964266,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17444991975366667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0352228720436665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 181.33240696026732,
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
        "date": 1746338703849,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09302986981400001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 132.15388405132035,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.076764121352,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.13692849878000002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.095086742655,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 129.81470588945467,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.183125203478,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1460106043683334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 138.47020848836863,
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
        "date": 1746943431054,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09536135426833332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 143.13442164331732,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.06709427176166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.15960841623133334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10678961406799999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 137.211339826893,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17720937865133335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.21043567775,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 145.278742642332,
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
        "date": 1747548182818,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09006969318366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 141.36114868321167,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.065970612892,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.180931795728,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.093129365798,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 147.03940301599832,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17978283507899998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1860713858103333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 152.51195138632,
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
        "date": 1748153079532,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09456266802,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 141.53643039914533,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09137677154633334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19377601398833333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.097265466852,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 134.09169243939064,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.179634991759,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1907648567406666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 134.6524251783463,
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
        "date": 1748758393696,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10246489100666667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 151.00573356763667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08761436979466668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.191564083042,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10116464912566665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 145.11223609008368,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17408579302566665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0434203310403334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 142.48010009491065,
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
        "date": 1749362765666,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08916170303733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 142.9831600749657,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09084671042999999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.165226449774,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09422752288199998,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 140.62885434846268,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.21643104244466668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.184066071465,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 140.41648007416399,
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
        "date": 1749967631198,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.093622780005,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 140.55291222752032,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08000080720766667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18227590165133334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.099546519811,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 137.03885906953232,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18418371856833335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.143665213764,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 138.59851983122832,
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
        "date": 1750572684131,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.101690358019,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 136.007274358589,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07887025810566665,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.1916316286666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09396773071966667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 143.10322739732234,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.163092596704,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1192018611576666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 141.95469703060166,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}