import { Sql } from "postgres";

export const API_KEY =
	"97b8772e8461cb1ddeae4776df27b1ff06ff2f5c4c674fe8a3a527451d820aa8";

interface SetupOutput {
	orgId: string;
	scenarioId: string;
	continuousScreeningConfigId: string;
}

export type { SetupOutput };

export const performInitialSetup = async (
	sql: Sql,
	apiUrl: string,
): Promise<SetupOutput> => {
	const [org] = await sql`select id from organizations limit 1`;
	const orgId = org.id;

	await sql`create publication alltables for all tables`;

	const result = await sql`
    insert into api_keys (id, org_id, prefix, role, key_hash, created_at)
    values (gen_random_uuid(), ${orgId}, 'nop', 4, decode('144347200CDEF84EB97250AEF807050611ABA445A85BEFB2593CD5D9054442D1', 'hex')::bytea, now())
  `;

	expect(result.count).toBe(1);

	await createDataModel(apiUrl);

	const scenarioId = await createScenario(apiUrl);
	const continuousScreeningConfigId =
		await createContinuousScreeningConfig(apiUrl);

	return { orgId, scenarioId, continuousScreeningConfigId };
};

const createDataModel = async (apiUrl: string) => {
	const dataModelTable = {
		name: "users",
		alias: "Users",
		semantic_type: "other",
		ftm_entity: "Person",
		fields: [
			{
				name: "object_id",
				alias: "object_id",
				type: "String",
				nullable: false,
			},
			{
				name: "name",
				alias: "Name",
				type: "String",
				ftm_property: "name",
			},
			{
				name: "updated_at",
				alias: "updated_at",
				type: "Timestamp",
				nullable: false,
			},
		],
	};

	const dataModelResponse = await fetch(`${apiUrl}/data-model/tables`, {
		method: "POST",
		headers: { "x-api-key": API_KEY },
		body: JSON.stringify(dataModelTable),
	});

	expect(dataModelResponse.status).toBe(201);
};

const createScenario = async (apiUrl: string): Promise<string> => {
	const scenarioResponse = await fetch(`${apiUrl}/scenarios`, {
		method: "POST",
		headers: { "x-api-key": API_KEY },
		body: JSON.stringify({
			name: "My Screening",
			trigger_object_type: "users",
		}),
	});

	expect(scenarioResponse.status).toBe(200);

	const scenarioId = (await scenarioResponse.json()).id;

	const iterationResponse = await fetch(`${apiUrl}/scenario-iterations`, {
		method: "POST",
		headers: { "x-api-key": API_KEY },
		body: JSON.stringify({
			scenario_id: scenarioId,
			body: {},
		}),
	});

	expect(iterationResponse.status).toBe(200);

	const iterationId = (await iterationResponse.json()).id;

	const screening = {
		name: "Screening",
		provider: "opensanctions",
		datasets: ["fr_assemblee"],
		entity_type: "Person",
		query: {
			name: {
				name: "StringConcat",
				children: [
					{
						name: "Payload",
						children: [
							{
								constant: "name",
							},
						],
					},
				],
				named_children: {
					with_separator: {
						constant: true,
					},
				},
			},
		},
	};

	const screeningResponse = await fetch(
		`${apiUrl}/scenario-iterations/${iterationId}/screening`,
		{
			method: "POST",
			headers: { "x-api-key": API_KEY },
			body: JSON.stringify(screening),
		},
	);

	expect(screeningResponse.status).toBe(200);

	expect(
		(
			await fetch(`${apiUrl}/scenario-iterations/${iterationId}/commit`, {
				method: "POST",
				headers: { "x-api-key": API_KEY },
			})
		).status,
	).toBe(200);

	expect(
		(
			await fetch(`${apiUrl}/scenario-publications`, {
				method: "POST",
				headers: { "x-api-key": API_KEY },
				body: JSON.stringify({
					scenario_iteration_id: iterationId,
					publication_action: "publish",
				}),
			})
		).status,
	).toBe(200);

	return scenarioId;
};

const createContinuousScreeningConfig = async (
	apiUrl: string,
): Promise<string> => {
	const inboxResponse = await fetch(`${apiUrl}/inboxes`, {
		method: "POST",
		headers: { "x-api-key": API_KEY, "content-type": "application/json" },
		body: JSON.stringify({ name: "Continuous Screening" }),
	});

	expect(inboxResponse.status).toBe(200);

	const inbox = await inboxResponse.json();

	const configResponse = await fetch(
		`${apiUrl}/continuous-screenings/configs`,
		{
			method: "POST",
			headers: { "x-api-key": API_KEY },
			body: JSON.stringify({
				name: "Continuous Screening Config",
				object_types: ["users"],
				datasets: ["fr_assemblee"],
				filters: {
					sanctions: {
						enabled: true,
						datasets: ["fr_assemblee"],
					},
				},
				match_threshold: 50,
				match_limit: 10,
				inbox_id: inbox.inbox.id,
			}),
		},
	);

	expect(configResponse.status).toBe(201);

	const config = await configResponse.json();

	return config.stable_id;
};
