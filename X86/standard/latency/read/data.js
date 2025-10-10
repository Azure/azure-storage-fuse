window.BENCHMARK_DATA = {
  "lastUpdate": 1760096209793,
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
        "date": 1751177932860,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09271918798366667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 150.41935688121933,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08671942833533335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.176795641225,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09731182387600001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 149.78328286088,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17054071454966668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1699569228176667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 144.0913143248657,
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
        "date": 1751782204907,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.08563313439733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 135.90314761989333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.077240082116,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.20498612222033333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09485363574733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 146.9487706477063,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.18038171551533333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.055302778124,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 140.929556464429,
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
        "date": 1752387350405,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09358328335033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 146.128216381439,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09199512903033334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.14895869399133335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10374445581733334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 139.020944262048,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.158930487998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0640023218616668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 137.01332924648764,
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
        "date": 1752992189475,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09706589319733333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 152.73527546274298,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07092153348966668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18699934019599998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09546093051633335,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 137.49850136184034,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.171489778491,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.033112949276,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 135.971156634619,
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
        "date": 1753596953755,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09723779204833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 167.192620486621,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08532659200233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.16573083938166666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09818607707466666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 149.67884338126797,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1787951389086667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.2126229523753336,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 149.59910293323767,
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
        "date": 1755411037807,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09395468463266667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 140.22966233142665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.077629330873,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.203819033298,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09963605687666666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 135.26429493390597,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.19290222410233335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0983309259750003,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 138.73168148190402,
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
        "date": 1756015578803,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09889763182866666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 144.11174446676264,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08865725516966666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18949461043933336,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09408568365499999,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 133.97940734130268,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17318016567966665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.1721586540043332,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 135.61773182013235,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "Copilot",
            "username": "Copilot",
            "email": "198982749+Copilot@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "ba815585e3ce3b2d08f0009de26c212e655af50c",
          "message": "Add comprehensive GitHub Copilot instructions for Azure Storage Fuse development (#1938)\n\nCo-authored-by: copilot-swe-agent[bot] <198982749+Copilot@users.noreply.github.com>\nCo-authored-by: vibhansa-msft <64532198+vibhansa-msft@users.noreply.github.com>",
          "timestamp": "2025-08-26T08:13:31Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/ba815585e3ce3b2d08f0009de26c212e655af50c"
        },
        "date": 1756620298003,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09254898676000001,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 149.767754885142,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.09588765912833334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18549463242533334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.10187466534166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 138.41054232194367,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.17655646509666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.2030387861976666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 148.088689903848,
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
          "id": "dfa3e9d92d4849695965058de77c287f9a0901ce",
          "message": "AI Comment cleanup (#1995)",
          "timestamp": "2025-09-18T11:22:08Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/dfa3e9d92d4849695965058de77c287f9a0901ce"
        },
        "date": 1758434450977,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.10451123878633334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 149.908659717093,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.07948047055166667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.13740452062033334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.095336896738,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 138.15912857639134,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.1750863070116667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9946566591676667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 138.41089083266166,
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
          "id": "9bada825b18507d8648fb3d5a4271e8374f57978",
          "message": "Updating go dependencies (#1972)",
          "timestamp": "2025-09-26T09:30:51Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/9bada825b18507d8648fb3d5a4271e8374f57978"
        },
        "date": 1759039255377,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.11442869331833333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 136.93147723691064,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.063668971738,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18406731523133332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09498468902233333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 131.804004058719,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.15571938660333332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9619238418593333,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 134.94000596990065,
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
          "id": "43314da664fe649d926fa148b6253ae28dff8d3f",
          "message": "Add FIO tests to check the data integrity (#1893)",
          "timestamp": "2025-09-29T10:20:04Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/43314da664fe649d926fa148b6253ae28dff8d3f"
        },
        "date": 1759644194960,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09635779608566668,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 135.37428673588934,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.08561769110466666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.19154740661399997,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09230373235366666,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 134.063453765947,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.172753741998,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 0.9883215639576667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 140.632292527182,
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
          "id": "389136cf285c96aae19b8e61c3c5bb0cee98bb45",
          "message": "Fix issues while truncating the file (#2003)\n\nCo-authored-by: Vikas Bhansali <64532198+vibhansa-msft@users.noreply.github.com>\nCo-authored-by: vibhansa <vibhansa@microsoft.com>",
          "timestamp": "2025-10-10T08:30:20Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/389136cf285c96aae19b8e61c3c5bb0cee98bb45"
        },
        "date": 1760096209391,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_read",
            "value": 0.09620742606566667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read",
            "value": 146.75045298776232,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_small_file",
            "value": 0.078325767039,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_small_file",
            "value": 0.18014210336866668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_direct_io",
            "value": 0.09700794690633334,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_direct_io",
            "value": 165.68671835772867,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_4_threads",
            "value": 0.15649563544300002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_read_16_threads",
            "value": 1.0604191415176667,
            "unit": "milliseconds"
          },
          {
            "name": "random_read_4_threads",
            "value": 140.99317003314567,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}