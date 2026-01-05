window.BENCHMARK_DATA = {
  "lastUpdate": 1767530792247,
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
        "date": 1741579069876,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.6789118680436665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 1.5601901562856666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.507205761729,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.7832747977576666,
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
        "date": 1742387661991,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.09526311218966665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12301933440766666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09324198536766666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09206107960666667,
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
        "date": 1744279960651,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.11731706696100001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.118614805903,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.11943601519933333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.11943968628233333,
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
        "date": 1744359007488,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "sequential_write",
            "value": 0.09232951532233334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11925237878799999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.104870528067,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09471263115733335,
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
        "date": 1744540401630,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 44.4807614,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09656743119966667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11974691956066667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09631870445233333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09451000767066665,
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
        "date": 1744634884614,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.0687304166666665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09255556682333332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12332734280833331,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.11728107375300001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09418288480433334,
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
        "date": 1745142825283,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.5960048666666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.095280630351,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12349309427166667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09799267579366666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09510330001233332,
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
        "date": 1745747430270,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.622003566666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.094626790177,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12307538417000001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09107127542866666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.098942341314,
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
        "date": 1746352827183,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 42.8149546,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.094547934542,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12953606025666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.090183625375,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09680120300533333,
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
        "date": 1746957489677,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.6384362833333332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.091239988775,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11547132410933332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09201783537333331,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.08749142909366668,
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
        "date": 1747562233674,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 164.59360955,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09021365725399999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12738176079100003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09175522097666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09367595982766667,
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
        "date": 1748167434772,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.6477605166666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09264900356466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.125702003486,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.094181496898,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09767680404566666,
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
        "date": 1748772758083,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 28.87468588333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09691250267966667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12108135996166665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09300744394933334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09163536069766669,
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
        "date": 1749376903807,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.727889283333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09642774617133333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.124721702882,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.093622647826,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09433848939733334,
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
        "date": 1749981586531,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 56.08663255,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.092605548809,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12211592310333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09228845934199999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09467776030933334,
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
        "date": 1750587160053,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 35.634322250000004,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09482193518533333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12395996090233334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09645319741933334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09567013032066667,
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
        "date": 1751196203026,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.755694483333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.10621990211900001,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12488576198366669,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09626025752566668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09933365086366668,
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
        "date": 1751796595991,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.7066057499999996,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09814658759966666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12622597387599999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09719016662666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09165712435366667,
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
        "date": 1752401803643,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.7021416166666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09840296015600002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.10925087552066666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.10064098072266665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09496759965033334,
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
        "date": 1753006563517,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 143.26789055,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09733202758333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11539197057266666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09699294397333331,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09803862165333332,
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
        "date": 1758448556203,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.821031416666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09457229613733333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.10589558897666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09924006375433332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09832613473233333,
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
        "date": 1759053775681,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.6374970166666665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.092347842611,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11210908368466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09077024913166666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09810000383266666,
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
        "date": 1759658916351,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.2699344499999996,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09875840039333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.10729250644933334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09001299053566668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.095694278329,
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
        "date": 1760116600610,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.48360575,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09187994425866668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.10924417780399999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09140018720333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09482564256933333,
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
        "date": 1760271184911,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.9164402000000003,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09285062360733333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11564078293700002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.092076726356,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09740023111333333,
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
          "id": "4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849",
          "message": "Updating release date (#2032)",
          "timestamp": "2025-10-16T04:32:59Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849"
        },
        "date": 1760875134230,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.4161613,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09363692127433333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.10936606938866666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09509937291733332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09847292372566667,
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
          "id": "4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849",
          "message": "Updating release date (#2032)",
          "timestamp": "2025-10-16T04:32:59Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849"
        },
        "date": 1761480678539,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.8899679166666665,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.093136333034,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.10948792083666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09333341898333335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.097222026965,
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
          "id": "4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849",
          "message": "Updating release date (#2032)",
          "timestamp": "2025-10-16T04:32:59Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/4b679ae9a43c3d6ff4aa5d744280dc5ef3aa5849"
        },
        "date": 1762084377738,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.865063166666667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09338992940799999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11619917821933334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.094914486814,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.10369486357733333,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "dependabot[bot]",
            "username": "dependabot[bot]",
            "email": "49699333+dependabot[bot]@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "421feb6dfe9ff7a89f7f224cb5af92f231539f18",
          "message": "Bump github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake from 1.4.2 to 1.4.3 (#2057)\n\nSigned-off-by: dependabot[bot] <support@github.com>\nCo-authored-by: dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
          "timestamp": "2025-11-07T08:58:35Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/421feb6dfe9ff7a89f7f224cb5af92f231539f18"
        },
        "date": 1762689770502,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.8747713166666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.095083429865,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.113747474706,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09466069576733334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09540734954366666,
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
          "id": "d2e3c8a69629afeda7e9b0d63074460fddbf8ca0",
          "message": "Adding mlperf scripts (#2061)",
          "timestamp": "2025-11-12T12:22:03Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d2e3c8a69629afeda7e9b0d63074460fddbf8ca0"
        },
        "date": 1763295564573,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.5618784666666663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09180522874699999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11447449653866666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.11255631449199999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.10807071232066667,
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
          "id": "f17c24583d29f72cf67eb652d27f482f87ecdc9f",
          "message": "Build support for arm32 (#2068)",
          "timestamp": "2025-11-21T09:53:19Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/f17c24583d29f72cf67eb652d27f482f87ecdc9f"
        },
        "date": 1763900482079,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.49677515,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.10429093852933334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11380482571466667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.10021162448566667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09768051285666668,
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
          "id": "a0590abf380af29afc9f050df470afb0f8b0a251",
          "message": "Gen-config command improvement (#2067)",
          "timestamp": "2025-11-28T11:25:13Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/a0590abf380af29afc9f050df470afb0f8b0a251"
        },
        "date": 1764505130422,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.0212029333333335,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09452149511566667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11883864164200002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09531027757499999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.1002678388,
            "unit": "milliseconds"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "name": "dependabot[bot]",
            "username": "dependabot[bot]",
            "email": "49699333+dependabot[bot]@users.noreply.github.com"
          },
          "committer": {
            "name": "GitHub",
            "username": "web-flow",
            "email": "noreply@github.com"
          },
          "id": "bf0bb76533a5215eec5c79e4a6ffbef4d2024a77",
          "message": "Bump github.com/spf13/cobra from 1.10.1 to 1.10.2 (#2085)\n\nSigned-off-by: dependabot[bot] <support@github.com>\nCo-authored-by: dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
          "timestamp": "2025-12-05T04:01:51Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/bf0bb76533a5215eec5c79e4a6ffbef4d2024a77"
        },
        "date": 1765109442171,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.9676680833333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09373286520633334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11644682298566666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09615263854733332,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09603715050333334,
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
          "id": "d8a31a5066f5f064fdce5de9fbf44006bf0693d5",
          "message": "Fix linting issues (#2087)",
          "timestamp": "2025-12-12T08:13:45Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/d8a31a5066f5f064fdce5de9fbf44006bf0693d5"
        },
        "date": 1765715608055,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 2.8150646666666663,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09649866857166667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.12870414957700002,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09372552318233333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.096771382143,
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
          "id": "dd6a9cf285ebcefc98cdc8ebc7405b889ba4c65e",
          "message": "Add goroutine id in debug logs (#2063)",
          "timestamp": "2025-12-17T09:18:20Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/dd6a9cf285ebcefc98cdc8ebc7405b889ba4c65e"
        },
        "date": 1766320382420,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.4174115166666668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09311292673499999,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11314644939166667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.095772989766,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09370001274499999,
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
          "id": "687ac7f12b8f119ff944acba16c1439838d8932e",
          "message": "Refactor tests (#2090)",
          "timestamp": "2025-12-24T09:11:18Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/687ac7f12b8f119ff944acba16c1439838d8932e"
        },
        "date": 1766925628634,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 21.575037933333334,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.08853150305866668,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11367713372033333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.092303455418,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09555642025233334,
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
          "id": "687ac7f12b8f119ff944acba16c1439838d8932e",
          "message": "Refactor tests (#2090)",
          "timestamp": "2025-12-24T09:11:18Z",
          "url": "https://github.com/Azure/azure-storage-fuse/commit/687ac7f12b8f119ff944acba16c1439838d8932e"
        },
        "date": 1767530792017,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "test",
            "value": 3.527612383333333,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write",
            "value": 0.09483105358666666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_directio",
            "value": 0.11204622728066667,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_4_threads",
            "value": 0.09495369088366666,
            "unit": "milliseconds"
          },
          {
            "name": "sequential_write_16_threads",
            "value": 0.09209287146833334,
            "unit": "milliseconds"
          }
        ]
      }
    ]
  }
}