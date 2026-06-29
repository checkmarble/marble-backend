import { Row, Sql } from "postgres";
import { API_KEY } from "./marble/setup";

export const testOutgoingContinuousMonitoring = async (
	sql: Sql,
	apiUrl: string,
	configId: string,
) => {
	let found = false;
	let foundCase: Row;

	const { unsubscribe: off } = await sql.subscribe(
		"insert:marble.cases",
		(row) => {
			found = true;
			foundCase = row as Row;
		},
	);

	let ingestion = await fetch(
		`${apiUrl}/v1/ingest/users?monitor=true&monitoring_config_id=${configId}`,
		{
			method: "POST",
			headers: { "x-api-key": API_KEY },
			body: JSON.stringify({
				object_id: "anything",
				updated_at: new Date().toISOString(),
				name: "Charles de Courson",
			}),
		},
	);

	expect(ingestion.status).toBe(201);

	while (!found) {
		await new Promise((resolve) => setTimeout(resolve, 1000));
	}

	expect(foundCase!.type).toBe("continuous_screening");
	expect(foundCase!.name).toBe("Charles de Courson");

	off();
};
