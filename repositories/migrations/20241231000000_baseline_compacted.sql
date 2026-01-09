-- +goose Up
-- +goose StatementBegin
-- =============================================================================
-- COMPACTED BASELINE MIGRATION
-- =============================================================================
-- This migration represents the compacted state of all migrations prior to
-- February 2025 (versions 20230515205456 through 20241218115400).
--
-- For FRESH INSTALLS: This migration creates the complete schema.
-- For EXISTING DATABASES: Goose will skip this (version is already higher).
-- =============================================================================
-- Create schema and configure search path
CREATE SCHEMA marble;

DO $$
BEGIN
   EXECUTE 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA marble TO ' || current_user;
END
$$;

DO $$
BEGIN
   EXECUTE 'ALTER DATABASE ' || current_database() || ' SET search_path TO marble, public';
END
$$;

DO $$
BEGIN
   EXECUTE format('ALTER ROLE %I SET search_path = marble, public;', current_user);
END
$$;

SET
    SEARCH_PATH = marble,
    public;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

--
-- Name: audit_operation; Type: TYPE; Schema: marble; Owner: -
--
CREATE TYPE marble.audit_operation AS ENUM('INSERT', 'UPDATE', 'DELETE');

--
-- Name: data_model_types; Type: TYPE; Schema: marble; Owner: -
--
CREATE TYPE marble.data_model_types AS ENUM('Bool', 'Int', 'Float', 'String', 'Timestamp');

--
-- Name: decision_outcome; Type: TYPE; Schema: marble; Owner: -
--
CREATE TYPE marble.decision_outcome AS ENUM('approve', 'decline', 'review', 'block_and_review', 'unknown');

--
-- Name: inbox_roles; Type: TYPE; Schema: marble; Owner: -
--
CREATE TYPE marble.inbox_roles AS ENUM('member', 'admin');

--
-- Name: inbox_status; Type: TYPE; Schema: marble; Owner: -
--
CREATE TYPE marble.inbox_status AS ENUM('active', 'archived');

--
-- Name: global_audit(); Type: FUNCTION; Schema: marble; Owner: -
--
CREATE FUNCTION marble.global_audit () RETURNS trigger LANGUAGE plpgsql AS $$
    BEGIN
        IF (TG_OP = 'DELETE') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('DELETE', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, OLD.id, to_jsonb(OLD), now());

        ELSIF (TG_OP = 'UPDATE') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('UPDATE', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());

        ELSIF (TG_OP = 'INSERT') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('INSERT', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());
        END IF;
        RETURN NULL;
    END;
$$;

--
-- Name: api_keys; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.api_keys (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        prefix character varying NOT NULL,
        deleted_at timestamp with time zone,
        role integer NOT NULL,
        description character varying(255) DEFAULT ''::character varying NOT NULL,
        key_hash bytea NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        partner_id uuid
    );

--
-- Name: case_contributors; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.case_contributors (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        case_id uuid NOT NULL,
        user_id uuid NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL
    );

--
-- Name: case_events; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.case_events (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        case_id uuid NOT NULL,
        user_id uuid,
        event_type character varying NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        additional_note text,
        resource_id uuid,
        resource_type text,
        new_value text,
        previous_value text
    );

--
-- Name: case_files; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.case_files (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        case_id uuid NOT NULL,
        bucket_name character varying(255) NOT NULL,
        file_reference character varying(255) NOT NULL,
        file_name character varying(255) NOT NULL
    );

--
-- Name: case_tags; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.case_tags (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        case_id uuid NOT NULL,
        tag_id uuid NOT NULL,
        created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
        deleted_at timestamp with time zone
    );

--
-- Name: cases; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.cases (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        name text NOT NULL,
        status character varying DEFAULT 'open'::character varying NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        inbox_id uuid
    );

--
-- Name: custom_list_values; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.custom_list_values (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        custom_list_id uuid NOT NULL,
        value character varying NOT NULL,
        created_at timestamp with time zone DEFAULT now(),
        deleted_at timestamp with time zone
    );

--
-- Name: custom_lists; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.custom_lists (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        organization_id uuid NOT NULL,
        name character varying NOT NULL,
        description character varying NOT NULL,
        created_at timestamp with time zone DEFAULT now(),
        updated_at timestamp with time zone DEFAULT now(),
        deleted_at timestamp with time zone
    );

--
-- Name: data_model_enum_values; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.data_model_enum_values (
        field_id uuid NOT NULL,
        value text,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        float_value double precision,
        text_value text
    );

--
-- Name: data_model_fields; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.data_model_fields (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        table_id uuid NOT NULL,
        name text NOT NULL,
        type marble.data_model_types NOT NULL,
        nullable boolean NOT NULL,
        description text,
        is_enum boolean DEFAULT false NOT NULL
    );

--
-- Name: data_model_links; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.data_model_links (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        organization_id uuid NOT NULL,
        name text NOT NULL,
        parent_table_id uuid NOT NULL,
        parent_field_id uuid NOT NULL,
        child_table_id uuid NOT NULL,
        child_field_id uuid NOT NULL
    );

--
-- Name: data_model_pivots; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.data_model_pivots (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        base_table_id uuid NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        field_id uuid,
        organization_id uuid NOT NULL,
        path_link_ids uuid[] DEFAULT ARRAY[]::uuid[] NOT NULL
    );

--
-- Name: data_model_tables; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.data_model_tables (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        organization_id uuid NOT NULL,
        name text NOT NULL,
        description text
    );

--
-- Name: decision_rules; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.decision_rules (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        decision_id uuid NOT NULL,
        score_modifier integer NOT NULL,
        result boolean NOT NULL,
        error_code integer NOT NULL,
        rule_id uuid NOT NULL,
        rule_evaluation jsonb,
        outcome character varying DEFAULT ''::character varying NOT NULL
    )
WITH
    (toast_tuple_target = '128');

--
-- Name: decisions; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.decisions (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        outcome marble.decision_outcome NOT NULL,
        scenario_id uuid NOT NULL,
        scenario_name character varying NOT NULL,
        scenario_description character varying NOT NULL,
        scenario_version integer NOT NULL,
        score integer NOT NULL,
        trigger_object_type character varying,
        trigger_object jsonb,
        scheduled_execution_id uuid,
        case_id uuid,
        scenario_iteration_id uuid NOT NULL,
        pivot_id uuid,
        pivot_value text,
        review_status character varying(10)
    );

--
-- Name: decisions_to_create; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.decisions_to_create (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        scheduled_execution_id uuid,
        object_id character varying(100) NOT NULL,
        status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
        created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
        updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
        CONSTRAINT decisions_to_create_status_check CHECK (
            (
                (status)::text = ANY (
                    (
                        ARRAY[
                            'pending'::character varying,
                            'created'::character varying,
                            'failed'::character varying,
                            'trigger_mismatch'::character varying,
                            'retry'::character varying
                        ]
                    )::text[]
                )
            )
        )
    );

--
-- Name: inbox_users; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.inbox_users (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        updated_at timestamp with time zone DEFAULT now() NOT NULL,
        inbox_id uuid NOT NULL,
        user_id uuid NOT NULL,
        role marble.inbox_roles DEFAULT 'member'::marble.inbox_roles NOT NULL
    );

--
-- Name: inboxes; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.inboxes (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        name character varying(255) NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        updated_at timestamp with time zone DEFAULT now() NOT NULL,
        organization_id uuid NOT NULL,
        status marble.inbox_status DEFAULT 'active'::marble.inbox_status NOT NULL
    );

--
-- Name: licenses; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.licenses (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        key character varying NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        suspended_at timestamp with time zone,
        expiration_date timestamp with time zone NOT NULL,
        name character varying NOT NULL,
        description character varying NOT NULL,
        sso_entitlement boolean NOT NULL,
        workflows_entitlement boolean NOT NULL,
        analytics_entitlement boolean NOT NULL,
        data_enrichment boolean NOT NULL,
        user_roles boolean NOT NULL,
        webhooks boolean DEFAULT false NOT NULL,
        rule_snoozes boolean DEFAULT false NOT NULL,
        test_run boolean DEFAULT false NOT NULL
    );

--
-- Name: organizations; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.organizations (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        name character varying NOT NULL,
        deleted_at timestamp with time zone,
        transfer_check_scenario_id uuid,
        use_marble_db_schema_as_default boolean DEFAULT false NOT NULL,
        default_scenario_timezone text
    );

--
-- Name: organizations_schema; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.organizations_schema (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid,
        schema_name character varying(255) NOT NULL
    );

--
-- Name: partners; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.partners (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        name character varying(255) NOT NULL,
        bic character varying DEFAULT ''::character varying NOT NULL
    );

--
-- Name: phantom_decisions; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.phantom_decisions (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        outcome marble.decision_outcome NOT NULL,
        scenario_id uuid NOT NULL,
        score integer NOT NULL,
        scenario_iteration_id uuid NOT NULL,
        scenario_version integer NOT NULL,
        test_run_id uuid NOT NULL
    );

--
-- Name: rule_snoozes; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.rule_snoozes (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        created_by_user uuid NOT NULL,
        snooze_group_id uuid NOT NULL,
        pivot_value text NOT NULL,
        starts_at timestamp with time zone NOT NULL,
        expires_at timestamp with time zone NOT NULL,
        created_from_decision_id uuid,
        created_from_rule_id uuid NOT NULL
    );

--
-- Name: scenario_iteration_rules; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.scenario_iteration_rules (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        scenario_iteration_id uuid NOT NULL,
        display_order smallint NOT NULL,
        name text NOT NULL,
        description text NOT NULL,
        score_modifier smallint NOT NULL,
        created_at timestamp with time zone DEFAULT now(),
        deleted_at timestamp with time zone,
        formula_ast_expression json,
        rule_group character varying(255) DEFAULT ''::character varying NOT NULL,
        snooze_group_id uuid,
        stable_rule_id uuid
    );

--
-- Name: scenario_iterations; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.scenario_iterations (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        scenario_id uuid NOT NULL,
        version smallint,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        updated_at timestamp with time zone DEFAULT now() NOT NULL,
        score_review_threshold smallint,
        score_reject_threshold smallint,
        deleted_at timestamp with time zone,
        schedule character varying DEFAULT ''::character varying,
        trigger_condition_ast_expression json,
        score_block_and_review_threshold smallint
    );

--
-- Name: scenario_publications; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.scenario_publications (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        rank SERIAL NOT NULL,
        org_id uuid NOT NULL,
        scenario_id uuid NOT NULL,
        scenario_iteration_id uuid NOT NULL,
        publication_action character varying NOT NULL,
        created_at timestamp with time zone DEFAULT now()
    );

--
-- Name: scenario_test_run; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.scenario_test_run (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        scenario_iteration_id uuid NOT NULL,
        live_scenario_iteration_id uuid NOT NULL,
        created_at timestamp with time zone DEFAULT now(),
        expires_at timestamp with time zone NOT NULL,
        status character varying NOT NULL
    );

--
-- Name: scenarios; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.scenarios (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        name character varying NOT NULL,
        description character varying NOT NULL,
        trigger_object_type character varying NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        deleted_at timestamp with time zone,
        live_scenario_iteration_id uuid,
        decision_to_case_inbox_id uuid,
        decision_to_case_outcomes character varying(50) [],
        decision_to_case_workflow_type character varying(255) DEFAULT 'DISABLED'::character varying NOT NULL
    );

--
-- Name: scheduled_executions; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.scheduled_executions (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        organization_id uuid NOT NULL,
        scenario_id uuid NOT NULL,
        scenario_iteration_id uuid NOT NULL,
        status character varying NOT NULL,
        started_at timestamp without time zone DEFAULT now() NOT NULL,
        finished_at timestamp without time zone,
        number_of_created_decisions integer DEFAULT 0 NOT NULL,
        manual boolean DEFAULT false NOT NULL,
        number_of_planned_decisions integer,
        number_of_evaluated_decisions integer DEFAULT 0 NOT NULL
    );

--
-- Name: snooze_groups; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.snooze_groups (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        organization_id uuid NOT NULL
    );

--
-- Name: tags; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.tags (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        name character varying(255) NOT NULL,
        color character varying(255) NOT NULL,
        created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
        updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
        deleted_at timestamp with time zone,
        org_id uuid NOT NULL
    );

--
-- Name: transfer_alerts; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.transfer_alerts (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        transfer_id uuid NOT NULL,
        organization_id uuid NOT NULL,
        sender_partner_id uuid NOT NULL,
        beneficiary_partner_id uuid NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        status character varying(255) DEFAULT 'pending'::character varying NOT NULL,
        message text NOT NULL,
        transfer_end_to_end_id character varying(255) NOT NULL,
        beneficiary_iban character varying(255) NOT NULL,
        sender_iban character varying(255) NOT NULL
    );

--
-- Name: transfer_mappings; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.transfer_mappings (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        organization_id uuid NOT NULL,
        client_transfer_id character varying(60) NOT NULL,
        partner_id uuid NOT NULL
    );

--
-- Name: upload_logs; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.upload_logs (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        org_id uuid NOT NULL,
        user_id uuid NOT NULL,
        file_name character varying NOT NULL,
        status character varying NOT NULL,
        started_at timestamp with time zone DEFAULT now() NOT NULL,
        finished_at timestamp with time zone,
        lines_processed integer DEFAULT 0 NOT NULL,
        table_name character varying DEFAULT ''::character varying NOT NULL,
        num_rows_ingested integer DEFAULT 0 NOT NULL
    );

--
-- Name: users; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.users (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        email character varying NOT NULL,
        role integer NOT NULL,
        organization_id uuid,
        first_name character varying,
        last_name character varying,
        deleted_at timestamp with time zone,
        partner_id uuid
    );

--
-- Name: webhook_events; Type: TABLE; Schema: marble; Owner: -
--
CREATE TABLE
    marble.webhook_events (
        id uuid DEFAULT marble.uuid_generate_v4 () NOT NULL,
        created_at timestamp with time zone DEFAULT now() NOT NULL,
        updated_at timestamp with time zone DEFAULT now() NOT NULL,
        retry_count integer DEFAULT 0 NOT NULL,
        delivery_status character varying NOT NULL,
        organization_id uuid NOT NULL,
        partner_id uuid,
        event_type character varying NOT NULL,
        event_data json
    );

--
-- Name: case_contributors case_contributors_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_contributors
ADD CONSTRAINT case_contributors_pkey PRIMARY KEY (id);

--
-- Name: case_events case_events_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_events
ADD CONSTRAINT case_events_pkey PRIMARY KEY (id);

--
-- Name: case_files case_files_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_files
ADD CONSTRAINT case_files_pkey PRIMARY KEY (id);

--
-- Name: case_tags case_tags_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_tags
ADD CONSTRAINT case_tags_pkey PRIMARY KEY (id);

--
-- Name: cases cases_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.cases
ADD CONSTRAINT cases_pkey PRIMARY KEY (id);

--
-- Name: organizations_schema client_tables_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.organizations_schema
ADD CONSTRAINT client_tables_pkey PRIMARY KEY (id);

--
-- Name: custom_list_values custom_list_values_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.custom_list_values
ADD CONSTRAINT custom_list_values_pkey PRIMARY KEY (id);

--
-- Name: custom_lists custom_lists_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.custom_lists
ADD CONSTRAINT custom_lists_pkey PRIMARY KEY (id);

--
-- Name: data_model_fields data_model_fields_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_fields
ADD CONSTRAINT data_model_fields_pkey PRIMARY KEY (id);

--
-- Name: data_model_fields data_model_fields_table_id_name_key; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_fields
ADD CONSTRAINT data_model_fields_table_id_name_key UNIQUE (table_id, name);

--
-- Name: data_model_links data_model_links_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_links
ADD CONSTRAINT data_model_links_pkey PRIMARY KEY (id);

--
-- Name: data_model_pivots data_model_pivots_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_pivots
ADD CONSTRAINT data_model_pivots_pkey PRIMARY KEY (id);

--
-- Name: data_model_tables data_model_tables_organization_id_name_key; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_tables
ADD CONSTRAINT data_model_tables_organization_id_name_key UNIQUE (organization_id, name);

--
-- Name: data_model_tables data_model_tables_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_tables
ADD CONSTRAINT data_model_tables_pkey PRIMARY KEY (id);

--
-- Name: decision_rules decision_rules_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.decision_rules
ADD CONSTRAINT decision_rules_pkey PRIMARY KEY (id);

--
-- Name: decisions decisions_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.decisions
ADD CONSTRAINT decisions_pkey PRIMARY KEY (id);

--
-- Name: decisions_to_create decisions_to_create_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.decisions_to_create
ADD CONSTRAINT decisions_to_create_pkey PRIMARY KEY (id);

--
-- Name: inbox_users inbox_users_inbox_id_user_id_key; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.inbox_users
ADD CONSTRAINT inbox_users_inbox_id_user_id_key UNIQUE (inbox_id, user_id);

--
-- Name: inbox_users inbox_users_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.inbox_users
ADD CONSTRAINT inbox_users_pkey PRIMARY KEY (id);

--
-- Name: inboxes inboxes_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.inboxes
ADD CONSTRAINT inboxes_pkey PRIMARY KEY (id);

--
-- Name: licenses licenses_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.licenses
ADD CONSTRAINT licenses_pkey PRIMARY KEY (id);

--
-- Name: organizations organizations_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.organizations
ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);

--
-- Name: partners partners_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.partners
ADD CONSTRAINT partners_pkey PRIMARY KEY (id);

--
-- Name: phantom_decisions phantom_decisions_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.phantom_decisions
ADD CONSTRAINT phantom_decisions_pkey PRIMARY KEY (id);

--
-- Name: rule_snoozes rule_snoozes_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.rule_snoozes
ADD CONSTRAINT rule_snoozes_pkey PRIMARY KEY (id);

--
-- Name: scenario_iteration_rules scenario_iteration_rules_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_iteration_rules
ADD CONSTRAINT scenario_iteration_rules_pkey PRIMARY KEY (id);

--
-- Name: scenario_iterations scenario_iterations_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_iterations
ADD CONSTRAINT scenario_iterations_pkey PRIMARY KEY (id);

--
-- Name: scenario_publications scenario_publications_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_publications
ADD CONSTRAINT scenario_publications_pkey PRIMARY KEY (id);

--
-- Name: scenario_test_run scenario_test_run_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_test_run
ADD CONSTRAINT scenario_test_run_pkey PRIMARY KEY (id);

--
-- Name: scenarios scenarios_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenarios
ADD CONSTRAINT scenarios_pkey PRIMARY KEY (id);

--
-- Name: scheduled_executions scheduled_executions_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scheduled_executions
ADD CONSTRAINT scheduled_executions_pkey PRIMARY KEY (id);

--
-- Name: snooze_groups snooze_groups_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.snooze_groups
ADD CONSTRAINT snooze_groups_pkey PRIMARY KEY (id);

--
-- Name: tags tags_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.tags
ADD CONSTRAINT tags_pkey PRIMARY KEY (id);

--
-- Name: api_keys tokens_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.api_keys
ADD CONSTRAINT tokens_pkey PRIMARY KEY (id);

--
-- Name: transfer_alerts transfer_alerts_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.transfer_alerts
ADD CONSTRAINT transfer_alerts_pkey PRIMARY KEY (id);

--
-- Name: transfer_mappings transfer_mappings_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.transfer_mappings
ADD CONSTRAINT transfer_mappings_pkey PRIMARY KEY (id);

--
-- Name: data_model_enum_values unique_data_model_enum_float_values_field_id_value; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_enum_values
ADD CONSTRAINT unique_data_model_enum_float_values_field_id_value UNIQUE (field_id, float_value);

--
-- Name: data_model_enum_values unique_data_model_enum_text_values_field_id_value; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_enum_values
ADD CONSTRAINT unique_data_model_enum_text_values_field_id_value UNIQUE (field_id, text_value);

--
-- Name: data_model_fields unique_data_model_fields_name; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_fields
ADD CONSTRAINT unique_data_model_fields_name UNIQUE (table_id, name);

--
-- Name: data_model_links unique_data_model_links; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_links
ADD CONSTRAINT unique_data_model_links UNIQUE (parent_table_id, parent_field_id, child_table_id, child_field_id);

--
-- Name: data_model_tables unique_data_model_tables_name; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_tables
ADD CONSTRAINT unique_data_model_tables_name UNIQUE (organization_id, name);

--
-- Name: upload_logs upload_logs_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.upload_logs
ADD CONSTRAINT upload_logs_pkey PRIMARY KEY (id);

--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.users
ADD CONSTRAINT users_pkey PRIMARY KEY (id);

--
-- Name: webhook_events webhook_events_pkey; Type: CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.webhook_events
ADD CONSTRAINT webhook_events_pkey PRIMARY KEY (id);

--
-- Name: apikeys_key_hash_index; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX apikeys_key_hash_index ON marble.api_keys USING btree (key_hash)
WHERE
    (deleted_at IS NULL);

--
-- Name: case_contributors_case_id_user_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX case_contributors_case_id_user_id_idx ON marble.case_contributors USING btree (case_id, user_id);

--
-- Name: case_event_case_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX case_event_case_id_idx ON marble.case_events USING btree (case_id, created_at DESC);

--
-- Name: case_files_unique_case_id_file_name; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX case_files_unique_case_id_file_name ON marble.case_files USING btree (case_id, bucket_name, file_reference);

--
-- Name: case_org_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX case_org_id_idx ON marble.cases USING btree (org_id, created_at DESC);

--
-- Name: case_status_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX case_status_idx ON marble.cases USING btree (org_id, status, created_at DESC);

--
-- Name: case_tags_unique_case_id_tag_id; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX case_tags_unique_case_id_tag_id ON marble.case_tags USING btree (case_id, tag_id)
WHERE
    (deleted_at IS NULL);

--
-- Name: cases_add_to_case_workflow_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX cases_add_to_case_workflow_idx ON marble.cases USING btree (org_id, inbox_id, id)
WHERE
    (
        (status)::text = ANY ((ARRAY['open'::character varying, 'investigating'::character varying])::text[])
    );

--
-- Name: custom_list_unique_name_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX custom_list_unique_name_idx ON marble.custom_lists USING btree (organization_id, name)
WHERE
    (deleted_at IS NULL);

--
-- Name: data_model_pivots_base_table_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX data_model_pivots_base_table_id_idx ON marble.data_model_pivots USING btree (organization_id, base_table_id);

--
-- Name: decision_object_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decision_object_id_idx ON marble.decisions USING btree (org_id, ((trigger_object ->> 'object_id'::text)));

--
-- Name: decision_pivot_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decision_pivot_id_idx ON marble.decisions USING btree (pivot_id);

--
-- Name: decision_rules_decisionid_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decision_rules_decisionid_idx ON marble.decision_rules USING btree (decision_id);

--
-- Name: decisions_add_to_case_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_add_to_case_idx ON marble.decisions USING btree (org_id, pivot_value, case_id)
WHERE
    (
        (pivot_value IS NOT NULL)
        AND (case_id IS NOT NULL)
    );

--
-- Name: decisions_by_org_id_index; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_by_org_id_index ON marble.decisions USING btree (org_id, created_at DESC) INCLUDE (scenario_id, outcome, trigger_object_type, case_id);

--
-- Name: decisions_case_id_idx_2; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_case_id_idx_2 ON marble.decisions USING btree (case_id, org_id)
WHERE
    (case_id IS NOT NULL);

--
-- Name: decisions_org_search_idx_with_case; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_org_search_idx_with_case ON marble.decisions USING btree (org_id, created_at DESC) INCLUDE (scenario_id, outcome, trigger_object_type, case_id, review_status)
WHERE
    (case_id IS NOT NULL);

--
-- Name: decisions_pivot_value_index; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_pivot_value_index ON marble.decisions USING btree (org_id, pivot_value, created_at DESC);

--
-- Name: decisions_scenario_iteration_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_scenario_iteration_id_idx ON marble.decisions USING btree (scenario_iteration_id);

--
-- Name: decisions_scheduled_execution_id_idx_3; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_scheduled_execution_id_idx_3 ON marble.decisions USING btree (scheduled_execution_id, created_at DESC)
WHERE
    (scheduled_execution_id IS NOT NULL);

--
-- Name: decisions_to_create_query_pending_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_to_create_query_pending_idx ON marble.decisions_to_create USING btree (scheduled_execution_id)
WHERE
    (
        (status)::text = ANY ((ARRAY['pending'::character varying, 'failed'::character varying])::text[])
    );

--
-- Name: decisions_to_create_unique_per_batch_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX decisions_to_create_unique_per_batch_idx ON marble.decisions_to_create USING btree (scheduled_execution_id, object_id) INCLUDE (status);

--
-- Name: idx_custom_list_id; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX idx_custom_list_id ON marble.custom_list_values USING btree (custom_list_id);

--
-- Name: idx_key; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX idx_key ON marble.licenses USING btree (key);

--
-- Name: idx_organization_id; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX idx_organization_id ON marble.custom_lists USING btree (organization_id);

--
-- Name: idx_table_name_org_id; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX idx_table_name_org_id ON marble.upload_logs USING btree (table_name, org_id);

--
-- Name: organization_name_unique_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX organization_name_unique_idx ON marble.organizations USING btree (name)
WHERE
    (deleted_at IS NULL);

--
-- Name: organization_schema_org_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX organization_schema_org_id_idx ON marble.organizations_schema USING btree (org_id);

--
-- Name: partners_bic_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX partners_bic_idx ON marble.partners USING btree (upper((bic)::text));

--
-- Name: phantom_decisions_org_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX phantom_decisions_org_idx ON marble.phantom_decisions USING btree (org_id, created_at DESC);

--
-- Name: rule_snoozes_by_pivot; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX rule_snoozes_by_pivot ON marble.rule_snoozes USING btree (pivot_value);

--
-- Name: scheduled_executions_organization_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX scheduled_executions_organization_id_idx ON marble.scheduled_executions USING btree (organization_id);

--
-- Name: scheduled_executions_scenario_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX scheduled_executions_scenario_id_idx ON marble.scheduled_executions USING btree (scenario_id);

--
-- Name: tags_unique_name_org_id; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX tags_unique_name_org_id ON marble.tags USING btree (name, org_id)
WHERE
    (deleted_at IS NULL);

--
-- Name: transfer_alerts_beneficiary_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX transfer_alerts_beneficiary_idx ON marble.transfer_alerts USING btree (organization_id, beneficiary_partner_id, created_at DESC);

--
-- Name: transfer_alerts_sender_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX transfer_alerts_sender_idx ON marble.transfer_alerts USING btree (organization_id, sender_partner_id, created_at DESC);

--
-- Name: transfer_alerts_unique_transfer_id; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX transfer_alerts_unique_transfer_id ON marble.transfer_alerts USING btree (transfer_id);

--
-- Name: transfer_mappings_client_transfer_id_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX transfer_mappings_client_transfer_id_idx ON marble.transfer_mappings USING btree (organization_id, partner_id, client_transfer_id);

--
-- Name: unique_scheduled_per_scenario_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX unique_scheduled_per_scenario_idx ON marble.scheduled_executions USING btree (scenario_id)
WHERE
    (
        (status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying])::text[])
    );

--
-- Name: users_email_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE UNIQUE INDEX users_email_idx ON marble.users USING btree (email)
WHERE
    (deleted_at IS NULL);

--
-- Name: users_organizationid_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX users_organizationid_idx ON marble.users USING btree (organization_id);

--
-- Name: webhooks_delivery_status_idx; Type: INDEX; Schema: marble; Owner: -
--
CREATE INDEX webhooks_delivery_status_idx ON marble.webhook_events USING btree (delivery_status)
WHERE
    (
        (delivery_status)::text = ANY ((ARRAY['scheduled'::character varying, 'retry'::character varying])::text[])
    );

--
-- Name: custom_list_values audit; Type: TRIGGER; Schema: marble; Owner: -
--
CREATE TRIGGER audit
AFTER INSERT
OR DELETE
OR
UPDATE ON marble.custom_list_values FOR EACH ROW
EXECUTE FUNCTION marble.global_audit ();

--
-- Name: api_keys api_keys_partner_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.api_keys
ADD CONSTRAINT api_keys_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES marble.partners (id) ON DELETE SET NULL;

--
-- Name: case_contributors case_contributors_case_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_contributors
ADD CONSTRAINT case_contributors_case_id_fkey FOREIGN KEY (case_id) REFERENCES marble.cases (id) ON DELETE CASCADE;

--
-- Name: case_contributors case_contributors_user_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_contributors
ADD CONSTRAINT case_contributors_user_id_fkey FOREIGN KEY (user_id) REFERENCES marble.users (id) ON DELETE CASCADE;

--
-- Name: case_events case_events_case_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_events
ADD CONSTRAINT case_events_case_id_fkey FOREIGN KEY (case_id) REFERENCES marble.cases (id) ON DELETE CASCADE;

--
-- Name: case_files case_files_case_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_files
ADD CONSTRAINT case_files_case_id_fkey FOREIGN KEY (case_id) REFERENCES marble.cases (id) ON DELETE CASCADE;

--
-- Name: case_tags case_tags_case_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_tags
ADD CONSTRAINT case_tags_case_id_fkey FOREIGN KEY (case_id) REFERENCES marble.cases (id) ON DELETE CASCADE;

--
-- Name: case_tags case_tags_tag_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.case_tags
ADD CONSTRAINT case_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES marble.tags (id) ON DELETE CASCADE;

--
-- Name: cases cases_inbox_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.cases
ADD CONSTRAINT cases_inbox_id_fkey FOREIGN KEY (inbox_id) REFERENCES marble.inboxes (id) ON DELETE CASCADE;

--
-- Name: cases cases_org_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.cases
ADD CONSTRAINT cases_org_id_fkey FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: data_model_fields data_model_fields_table_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_fields
ADD CONSTRAINT data_model_fields_table_id_fkey FOREIGN KEY (table_id) REFERENCES marble.data_model_tables (id) ON DELETE CASCADE;

--
-- Name: data_model_links data_model_links_child_field_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_links
ADD CONSTRAINT data_model_links_child_field_id_fkey FOREIGN KEY (child_field_id) REFERENCES marble.data_model_fields (id) ON DELETE CASCADE;

--
-- Name: data_model_links data_model_links_child_table_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_links
ADD CONSTRAINT data_model_links_child_table_id_fkey FOREIGN KEY (child_table_id) REFERENCES marble.data_model_tables (id) ON DELETE CASCADE;

--
-- Name: data_model_links data_model_links_organization_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_links
ADD CONSTRAINT data_model_links_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: data_model_links data_model_links_parent_field_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_links
ADD CONSTRAINT data_model_links_parent_field_id_fkey FOREIGN KEY (parent_field_id) REFERENCES marble.data_model_fields (id) ON DELETE CASCADE;

--
-- Name: data_model_links data_model_links_parent_table_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_links
ADD CONSTRAINT data_model_links_parent_table_id_fkey FOREIGN KEY (parent_table_id) REFERENCES marble.data_model_tables (id) ON DELETE CASCADE;

--
-- Name: data_model_pivots data_model_pivots_base_table_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_pivots
ADD CONSTRAINT data_model_pivots_base_table_id_fkey FOREIGN KEY (base_table_id) REFERENCES marble.data_model_tables (id) ON DELETE CASCADE;

--
-- Name: data_model_pivots data_model_pivots_field_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_pivots
ADD CONSTRAINT data_model_pivots_field_id_fkey FOREIGN KEY (field_id) REFERENCES marble.data_model_fields (id) ON DELETE CASCADE;

--
-- Name: data_model_pivots data_model_pivots_organization_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_pivots
ADD CONSTRAINT data_model_pivots_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: data_model_tables data_model_tables_organization_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.data_model_tables
ADD CONSTRAINT data_model_tables_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: decisions decisions_pivot_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.decisions
ADD CONSTRAINT decisions_pivot_id_fkey FOREIGN KEY (pivot_id) REFERENCES marble.data_model_pivots (id) ON DELETE SET NULL;

--
-- Name: decisions decisions_scenario_iteration_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.decisions
ADD CONSTRAINT decisions_scenario_iteration_id_fkey FOREIGN KEY (scenario_iteration_id) REFERENCES marble.scenario_iterations (id);

--
-- Name: decisions_to_create decisions_to_create_scheduled_execution_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.decisions_to_create
ADD CONSTRAINT decisions_to_create_scheduled_execution_id_fkey FOREIGN KEY (scheduled_execution_id) REFERENCES marble.scheduled_executions (id) ON DELETE SET NULL;

--
-- Name: custom_lists fk_custom_lists_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.custom_lists
ADD CONSTRAINT fk_custom_lists_org FOREIGN KEY (organization_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: custom_list_values fk_custom_lists_value_lists; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.custom_list_values
ADD CONSTRAINT fk_custom_lists_value_lists FOREIGN KEY (custom_list_id) REFERENCES marble.custom_lists (id) ON DELETE CASCADE;

--
-- Name: decisions fk_decisions_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.decisions
ADD CONSTRAINT fk_decisions_org FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: inbox_users fk_inbox_users_inbox; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.inbox_users
ADD CONSTRAINT fk_inbox_users_inbox FOREIGN KEY (inbox_id) REFERENCES marble.inboxes (id) ON DELETE CASCADE;

--
-- Name: inbox_users fk_inbox_users_user; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.inbox_users
ADD CONSTRAINT fk_inbox_users_user FOREIGN KEY (user_id) REFERENCES marble.users (id) ON DELETE CASCADE;

--
-- Name: inboxes fk_inboxes_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.inboxes
ADD CONSTRAINT fk_inboxes_org FOREIGN KEY (organization_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: organizations_schema fk_organization_schema_organization; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.organizations_schema
ADD CONSTRAINT fk_organization_schema_organization FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: phantom_decisions fk_phantom_decisions_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.phantom_decisions
ADD CONSTRAINT fk_phantom_decisions_org FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: phantom_decisions fk_phantom_decisions_scenario_ite_id; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.phantom_decisions
ADD CONSTRAINT fk_phantom_decisions_scenario_ite_id FOREIGN KEY (scenario_iteration_id) REFERENCES marble.scenario_iterations (id) ON DELETE CASCADE;

--
-- Name: scenario_iteration_rules fk_scenario_iteration_rules_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_iteration_rules
ADD CONSTRAINT fk_scenario_iteration_rules_org FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: scenario_iteration_rules fk_scenario_iteration_rules_scenario_iterations; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_iteration_rules
ADD CONSTRAINT fk_scenario_iteration_rules_scenario_iterations FOREIGN KEY (scenario_iteration_id) REFERENCES marble.scenario_iterations (id) ON DELETE CASCADE;

--
-- Name: scenario_iterations fk_scenario_iterations_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_iterations
ADD CONSTRAINT fk_scenario_iterations_org FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: scenario_iterations fk_scenario_iterations_scenarios; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_iterations
ADD CONSTRAINT fk_scenario_iterations_scenarios FOREIGN KEY (scenario_id) REFERENCES marble.scenarios (id) ON DELETE CASCADE;

--
-- Name: scenario_publications fk_scenario_publications_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_publications
ADD CONSTRAINT fk_scenario_publications_org FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: scenario_publications fk_scenario_publications_scenario_id; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_publications
ADD CONSTRAINT fk_scenario_publications_scenario_id FOREIGN KEY (scenario_id) REFERENCES marble.scenarios (id) ON DELETE CASCADE;

--
-- Name: scenario_publications fk_scenario_publications_scenario_iterations; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_publications
ADD CONSTRAINT fk_scenario_publications_scenario_iterations FOREIGN KEY (scenario_iteration_id) REFERENCES marble.scenario_iterations (id) ON DELETE CASCADE;

--
-- Name: scenario_test_run fk_scenario_publications_scenario_iterations; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_test_run
ADD CONSTRAINT fk_scenario_publications_scenario_iterations FOREIGN KEY (scenario_iteration_id) REFERENCES marble.scenario_iterations (id) ON DELETE CASCADE;

--
-- Name: scenarios fk_scenarios_live_scenario_iteration; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenarios
ADD CONSTRAINT fk_scenarios_live_scenario_iteration FOREIGN KEY (live_scenario_iteration_id) REFERENCES marble.scenario_iterations (id) ON DELETE CASCADE;

--
-- Name: scenarios fk_scenarios_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenarios
ADD CONSTRAINT fk_scenarios_org FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: api_keys fk_tokens_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.api_keys
ADD CONSTRAINT fk_tokens_org FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: webhook_events fk_webhooks_org; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.webhook_events
ADD CONSTRAINT fk_webhooks_org FOREIGN KEY (organization_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: webhook_events fk_webhooks_partner; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.webhook_events
ADD CONSTRAINT fk_webhooks_partner FOREIGN KEY (partner_id) REFERENCES marble.partners (id) ON DELETE CASCADE;

--
-- Name: organizations organizations_transfer_check_scenario_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.organizations
ADD CONSTRAINT organizations_transfer_check_scenario_id_fkey FOREIGN KEY (transfer_check_scenario_id) REFERENCES marble.scenarios (id) ON DELETE SET NULL;

--
-- Name: phantom_decisions phantom_decisions_test_run_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.phantom_decisions
ADD CONSTRAINT phantom_decisions_test_run_id_fkey FOREIGN KEY (test_run_id) REFERENCES marble.scenario_test_run (id) ON DELETE CASCADE;

--
-- Name: rule_snoozes rule_snoozes_created_by_user_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.rule_snoozes
ADD CONSTRAINT rule_snoozes_created_by_user_fkey FOREIGN KEY (created_by_user) REFERENCES marble.users (id);

--
-- Name: rule_snoozes rule_snoozes_created_from_decision_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.rule_snoozes
ADD CONSTRAINT rule_snoozes_created_from_decision_id_fkey FOREIGN KEY (created_from_decision_id) REFERENCES marble.decisions (id) ON DELETE SET NULL;

--
-- Name: rule_snoozes rule_snoozes_created_from_rule_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.rule_snoozes
ADD CONSTRAINT rule_snoozes_created_from_rule_id_fkey FOREIGN KEY (created_from_rule_id) REFERENCES marble.scenario_iteration_rules (id) ON DELETE CASCADE;

--
-- Name: rule_snoozes rule_snoozes_snooze_group_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.rule_snoozes
ADD CONSTRAINT rule_snoozes_snooze_group_id_fkey FOREIGN KEY (snooze_group_id) REFERENCES marble.snooze_groups (id);

--
-- Name: scenario_iteration_rules scenario_iteration_rules_snooze_group_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenario_iteration_rules
ADD CONSTRAINT scenario_iteration_rules_snooze_group_id_fkey FOREIGN KEY (snooze_group_id) REFERENCES marble.snooze_groups (id);

--
-- Name: scenarios scenarios_decision_to_case_inbox_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.scenarios
ADD CONSTRAINT scenarios_decision_to_case_inbox_id_fkey FOREIGN KEY (decision_to_case_inbox_id) REFERENCES marble.inboxes (id) ON UPDATE CASCADE ON DELETE SET NULL;

--
-- Name: snooze_groups snooze_groups_organization_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.snooze_groups
ADD CONSTRAINT snooze_groups_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES marble.organizations (id);

--
-- Name: tags tags_org_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.tags
ADD CONSTRAINT tags_org_id_fkey FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: transfer_alerts transfer_alerts_beneficiary_partner_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.transfer_alerts
ADD CONSTRAINT transfer_alerts_beneficiary_partner_id_fkey FOREIGN KEY (beneficiary_partner_id) REFERENCES marble.partners (id);

--
-- Name: transfer_alerts transfer_alerts_organization_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.transfer_alerts
ADD CONSTRAINT transfer_alerts_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES marble.organizations (id);

--
-- Name: transfer_alerts transfer_alerts_sender_partner_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.transfer_alerts
ADD CONSTRAINT transfer_alerts_sender_partner_id_fkey FOREIGN KEY (sender_partner_id) REFERENCES marble.partners (id);

--
-- Name: transfer_alerts transfer_alerts_transfer_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.transfer_alerts
ADD CONSTRAINT transfer_alerts_transfer_id_fkey FOREIGN KEY (transfer_id) REFERENCES marble.transfer_mappings (id);

--
-- Name: transfer_mappings transfer_mappings_organization_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.transfer_mappings
ADD CONSTRAINT transfer_mappings_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: transfer_mappings transfer_mappings_partner_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.transfer_mappings
ADD CONSTRAINT transfer_mappings_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES marble.partners (id) ON DELETE SET NULL;

--
-- Name: upload_logs upload_logs_org_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.upload_logs
ADD CONSTRAINT upload_logs_org_id_fkey FOREIGN KEY (org_id) REFERENCES marble.organizations (id) ON DELETE CASCADE;

--
-- Name: users users_partner_id_fkey; Type: FK CONSTRAINT; Schema: marble; Owner: -
--
ALTER TABLE ONLY marble.users
ADD CONSTRAINT users_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES marble.partners (id) ON DELETE CASCADE;

-- audit schema
CREATE SCHEMA audit;

--
-- Name: audit_events; Type: TABLE; Schema: audit; Owner: -
--
CREATE TABLE
    audit.audit_events (
        id uuid DEFAULT gen_random_uuid () NOT NULL,
        operation marble.audit_operation NOT NULL,
        user_id text,
        "table" character varying NOT NULL,
        entity_id uuid NOT NULL,
        data jsonb DEFAULT '{}'::jsonb NOT NULL,
        created_at timestamp(6)
        with
            time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
    );

--
-- Name: audit_events audit_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--
ALTER TABLE ONLY audit.audit_events
ADD CONSTRAINT audit_pkey PRIMARY KEY (id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- WARNING: This will drop the entire marble schema and all data!
-- Only use for rolling back a fresh install.
DROP SCHEMA IF EXISTS marble CASCADE;

DROP SCHEMA IF EXISTS audit CASCADE;

-- +goose StatementEnd