import { Row, Sql } from "postgres";
import { API_KEY } from "./marble/setup";
import { StartedNetwork, StartedTestContainer } from "testcontainers";
import { uploadDelta } from "./marble/catalog";
import { DEFAULT_MANIFEST, triggerIndexing } from "./marble/screening";

export const testIncomingContinuousMonitoring = async (
	network: StartedNetwork,
	sql: Sql,
	s3: StartedTestContainer,
	motiva: StartedTestContainer,
	apiUrl: string,
	configId: string,
) => {
	{
		let found = false;

		const { unsubscribe: off } = await sql.subscribe(
			"*:marble.continuous_screening_dataset_files",
			(row) => {
				if (row!.file_type === "full") {
					found = true;
				}
			},
		);

		let ingestion = await fetch(
			`${apiUrl}/v1/ingest/users?monitor=true&monitoring_config_id=${configId}`,
			{
				method: "POST",
				headers: { "x-api-key": API_KEY },
				body: JSON.stringify({
					object_id: "chervieu",
					updated_at: new Date().toISOString(),
					name: "Céline Hervieu",
				}),
			},
		);

		expect(ingestion.status).toBe(201);

		while (!found) {
			await new Promise((resolve) => setTimeout(resolve, 1000));
		}

		off();
	}

	const manifest = {
		catalogs: [
			{
				url: "http://api:8080/screening-indexer/catalogs",
				resource_name: "entities.ftm.json",
				auth_token: "authtoken",
			},
			...DEFAULT_MANIFEST.catalogs,
		],
	};

	await uploadDelta(network, s3);
	await triggerIndexing(network, manifest);

	await motiva.copyContentToContainer([
		{
			target: "/manifest.json",
			content: JSON.stringify(manifest),
		},
	]);

	await motiva.restart();

	let found = false;
	let foundCase: Row;

	const { unsubscribe: off } = await sql.subscribe("*:marble.cases", (row) => {
		console.log(row);
		found = true;
		foundCase = row as Row;
	});

	while (!found) {
		await new Promise((resolve) => setTimeout(resolve, 1000));
	}

	expect(foundCase!.type).toBe("continuous_screening");
	expect(foundCase!.name).toBe("Céline Hervieu");
	expect(foundCase!.status).toBe("pending");

	off();
};
