Sort  (cost=200461.18..200461.37 rows=77 width=200) (actual time=15264.887..15271.382 rows=0 loops=1)
  Sort Key: (min(sc.created_at)), sc.id
  Sort Method: quicksort  Memory: 25kB
  Buffers: shared hit=3401165
  ->  GroupAggregate  (cost=199800.03..200458.77 rows=77 width=200) (actual time=15264.881..15271.375 rows=0 loops=1)
        Group Key: sc.id
        Buffers: shared hit=3401165
        ->  Sort  (cost=199800.03..199800.23 rows=77 width=128) (actual time=15264.877..15271.370 rows=0 loops=1)
              Sort Key: sc.id
              Sort Method: quicksort  Memory: 25kB
              Buffers: shared hit=3401165
              ->  Nested Loop  (cost=1021.80..199797.62 rows=77 width=128) (actual time=15264.872..15271.364 rows=0 loops=1)
                    Buffers: shared hit=3401165
                    ->  Nested Loop  (cost=1021.23..199435.90 rows=77 width=128) (actual time=15264.871..15271.362 rows=0 loops=1)
                          Buffers: shared hit=3401165
                          ->  Subquery Scan on sc  (cost=1021.07..199187.36 rows=77 width=107) (actual time=15264.870..15271.358 rows=0 loops=1)
                                Filter: ((sc.created_at < '2025-11-30 17:37:30.59033+00'::timestamp with time zone) AND (sc.org_id = 'd45e546b-1e79-4c70-948e-c047e916c9d8'::uuid) AND ((sc.trigger_object_type)::text = 'account_holders'::text))
                                Buffers: shared hit=3401165
                                ->  Limit  (cost=1021.07..199012.36 rows=10000 width=107) (actual time=15264.868..15271.356 rows=0 loops=1)
                                      Buffers: shared hit=3401165
                                      ->  Incremental Sort  (cost=1021.07..905742.25 rows=45695 width=107) (actual time=15264.867..15271.354 rows=0 loops=1)
                                            Sort Key: sc_1.created_at, sc_1.id
                                            Presorted Key: sc_1.created_at
                                            Full-sort Groups: 1  Sort Method: quicksort  Average Memory: 25kB  Peak Memory: 25kB
                                            Buffers: shared hit=3401165
                                            ->  Gather Merge  (cost=1001.31..903685.97 rows=45695 width=107) (actual time=15264.860..15271.346 rows=0 loops=1)
                                                  Workers Planned: 2
                                                  Workers Launched: 2
                                                  Buffers: shared hit=3401165
                                                  ->  Nested Loop  (cost=1.28..897411.61 rows=19040 width=107) (actual time=15252.526..15252.530 rows=0 loops=3)
                                                        Buffers: shared hit=3401165
                                                        ->  Nested Loop  (cost=1.00..837021.34 rows=2411691 width=95) (actual time=0.501..12855.826 rows=1945090 loops=3)
                                                              Buffers: shared hit=3401145
                                                              ->  Nested Loop  (cost=0.71..776574.77 rows=2411691 width=95) (actual time=0.468..9637.646 rows=1945090 loops=3)
                                                                    Buffers: shared hit=3400942
                                                                    ->  Parallel Index Scan Backward using idx_screenings_org_id on screenings sc_1  (cost=0.56..716386.71 rows=2411691 width=79) (actual time=0.420..6033.282 rows=1945090 loops=3)
                                                                          Index Cond: ((org_id = 'd45e546b-1e79-4c70-948e-c047e916c9d8'::uuid) AND (created_at < '2025-11-30 17:37:30.59033+00'::timestamp with time zone) AND (created_at >= '0001-01-01 00:00:00+00'::timestamp with time zone))
                                                                          Filter: (ROW(created_at, id) > ROW('0001-01-01 00:00:00+00'::timestamp with time zone, '00000000-0000-0000-0000-000000000000'::uuid))
                                                                          Buffers: shared hit=3400740
                                                                    ->  Memoize  (cost=0.16..0.18 rows=1 width=32) (actual time=0.001..0.001 rows=1 loops=5835269)
                                                                          Cache Key: sc_1.screening_config_id
                                                                          Cache Mode: logical
                                                                          Hits: 1974045  Misses: 34  Evictions: 0  Overflows: 0  Memory Usage: 5kB
                                                                          Buffers: shared hit=202
                                                                          Worker 0:  Hits: 1908711  Misses: 33  Evictions: 0  Overflows: 0  Memory Usage: 5kB
                                                                          Worker 1:  Hits: 1952413  Misses: 33  Evictions: 0  Overflows: 0  Memory Usage: 5kB
                                                                          ->  Index Scan using sanction_check_configs_pkey on screening_configs scc_1  (cost=0.15..0.17 rows=1 width=32) (actual time=0.006..0.006 rows=1 loops=100)
                                                                                Index Cond: (id = sc_1.screening_config_id)
                                                                                Buffers: shared hit=202
                                                              ->  Memoize  (cost=0.29..0.97 rows=1 width=32) (actual time=0.001..0.001 rows=1 loops=5835269)
                                                                    Cache Key: scc_1.scenario_iteration_id
                                                                    Cache Mode: logical
                                                                    Hits: 1974056  Misses: 23  Evictions: 0  Overflows: 0  Memory Usage: 4kB
                                                                    Buffers: shared hit=203
                                                                    Worker 0:  Hits: 1908722  Misses: 22  Evictions: 0  Overflows: 0  Memory Usage: 4kB
                                                                    Worker 1:  Hits: 1952424  Misses: 22  Evictions: 0  Overflows: 0  Memory Usage: 4kB
                                                                    ->  Index Scan using scenario_iterations_pkey on scenario_iterations si  (cost=0.28..0.96 rows=1 width=32) (actual time=0.006..0.006 rows=1 loops=67)
                                                                          Index Cond: (id = scc_1.scenario_iteration_id)
                                                                          Buffers: shared hit=203
                                                        ->  Memoize  (cost=0.28..0.39 rows=1 width=28) (actual time=0.001..0.001 rows=0 loops=5835269)
                                                              Cache Key: si.scenario_id
                                                              Cache Mode: logical
                                                              Hits: 1974077  Misses: 2  Evictions: 0  Overflows: 0  Memory Usage: 1kB
                                                              Buffers: shared hit=20
                                                              Worker 0:  Hits: 1908742  Misses: 2  Evictions: 0  Overflows: 0  Memory Usage: 1kB
                                                              Worker 1:  Hits: 1952444  Misses: 2  Evictions: 0  Overflows: 0  Memory Usage: 1kB
                                                              ->  Index Scan using scenarios_pkey on scenarios s  (cost=0.27..0.38 rows=1 width=28) (actual time=0.012..0.012 rows=0 loops=6)
                                                                    Index Cond: (id = si.scenario_id)
                                                                    Filter: ((trigger_object_type)::text = 'account_holders'::text)
                                                                    Rows Removed by Filter: 1
                                                                    Buffers: shared hit=20
                          ->  Memoize  (cost=0.16..5.01 rows=1 width=53) (never executed)
                                Cache Key: sc.screening_config_id
                                Cache Mode: logical
                                ->  Index Scan using sanction_check_configs_pkey on screening_configs scc  (cost=0.15..5.00 rows=1 width=53) (never executed)
                                      Index Cond: (id = sc.screening_config_id)
                    ->  Memoize  (cost=0.57..4.74 rows=1 width=16) (never executed)
                          Cache Key: sc.decision_id
                          Cache Mode: logical
                          ->  Index Only Scan using decisions_pkey on decisions d  (cost=0.56..4.73 rows=1 width=16) (never executed)
                                Index Cond: (id = sc.decision_id)
                                Heap Fetches: 0
        SubPlan 1
          ->  Aggregate  (cost=8.46..8.47 rows=1 width=8) (never executed)
                ->  Index Only Scan using idx_screening_matches_screening_id on screening_matches m  (cost=0.42..8.46 rows=2 width=0) (never executed)
                      Index Cond: (screening_id = sc.id)
                      Heap Fetches: 0"