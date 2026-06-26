import { Sql } from "postgres";

export const API_KEY =
	"97b8772e8461cb1ddeae4776df27b1ff06ff2f5c4c674fe8a3a527451d820aa8";

interface SetupOutput {
	orgId: string;
	scenarioId: string;
}

export type { SetupOutput };

export const performInitialSetup = async (
	sql: Sql,
	apiUrl: string,
): Promise<SetupOutput> => {
	const [org] = await sql`select id from organizations limit 1`;
	const orgId = org.id;

	const result = await sql`
    insert into api_keys (id, org_id, prefix, role, key_hash, created_at)
    values (gen_random_uuid(), ${orgId}, 'nop', 4, decode('144347200CDEF84EB97250AEF807050611ABA445A85BEFB2593CD5D9054442D1', 'hex')::bytea, now())
  `;

	expect(result.count).toBe(1);

	await createDataModel(apiUrl);

	const scenarioId = await createScenario(apiUrl);

	return { orgId, scenarioId };
};

const createDataModel = async (apiUrl: string) => {
	const dataModelTable = {
		name: "users",
		alias: "Users",
		semantic_type: "other",
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
