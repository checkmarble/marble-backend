with
    "configs" as (
        SELECT
            scc.id AS screening_config_id,
            scc.stable_id,
            scc.name,
            s.id AS scenario_id,
            s.trigger_object_type
        FROM
            scenarios AS s
            INNER JOIN scenario_iterations AS si ON si.scenario_id = s.id
            INNER JOIN screening_configs AS scc ON scc.scenario_iteration_id = si.id
        WHERE
            s.org_id = $1
            AND s.trigger_object_type = $2
    ),
    "screenings_by_config" as (
        SELECT
            scs.*,
            configs.name AS config_name,
            configs.stable_id AS config_stable_id
        FROM
            configs
            CROSS JOIN LATERAL (
                SELECT
                    scs.id,
                    scs.decision_id,
                    scs.created_at,
                    scs.status,
                    scs.org_id,
                    configs.scenario_id,
                    configs.trigger_object_type
                FROM
                    screenings AS scs
                WHERE
                    scs.screening_config_id = configs.screening_config_id
                    AND scs.created_at < $3
                    AND (scs.created_at, scs.id) > ($4::timestamp with time zone, $5)
                ORDER BY
                    scs.created_at,
                    scs.id
                LIMIT
                    10000
            ) as scs
    ),
    "limited_screenings" as (
        SELECT
            *
        FROM
            screenings_by_config
        ORDER BY
            created_at,
            id
        LIMIT
            10000
    )
SELECT
    limited_screenings.id,
    MIN(limited_screenings.decision_id::text)::uuid AS decision_id,
    MIN(limited_screenings.status) AS status,
    MIN(limited_screenings.scenario_id::text)::uuid AS scenario_id,
    MIN(limited_screenings.created_at) AS created_at,
    MIN(limited_screenings.org_id::text)::uuid AS org_id,
    EXTRACT(
        year
        FROM
            MIN(limited_screenings.created_at)
    )::int AS year,
    EXTRACT(
        month
        FROM
            MIN(limited_screenings.created_at)
    )::int AS month,
    MIN(limited_screenings.trigger_object_type) AS trigger_object_type,
    MIN(limited_screenings.config_stable_id::text)::uuid AS screening_config_id,
    MIN(limited_screenings.config_name) AS screening_name,
    (
        SELECT
            count(*)
        FROM
            screening_matches m
        WHERE
            m.screening_id = limited_screenings.id
    ) AS matches
FROM
    limited_screenings
    INNER JOIN decisions AS d ON d.id = limited_screenings.decision_id
GROUP BY
    limited_screenings.id