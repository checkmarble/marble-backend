import { StartedPostgreSqlContainer } from "@testcontainers/postgresql";
import {
	startFirebase,
	startApi,
	startDatabase,
	startWorker,
	startS3,
} from "./marble/base";
import { Network, StartedNetwork, StartedTestContainer } from "testcontainers";
import { performInitialSetup, type SetupOutput } from "./marble/setup";
import postgres, { Sql } from "postgres";
import { startElasticsearch, startMotiva } from "./marble/screening";
import { testTransactionMonitoring } from "./transaction";
import { buildMarble } from "./marble/build";
import { uri } from "./marble/utils";
import { testOutgoingContinuousMonitoring } from "./continuous-outgoing";
import { testIncomingContinuousMonitoring } from "./continuous-incoming";
import { createFakeCatalog } from "./marble/catalog";

var network: StartedNetwork;

var fb: StartedTestContainer;
var pg: StartedPostgreSqlContainer;
var es: StartedTestContainer;
var s3: StartedTestContainer;
var motiva: StartedTestContainer;
var api: StartedTestContainer;
var worker: StartedTestContainer;

var sql: Sql;

var vars: SetupOutput;

beforeAll(
	async () => {
		network = await new Network().start();

		s3 = await startS3(network);

		await createFakeCatalog(network, s3);

		[, pg, fb, es] = await Promise.all([
			buildMarble(),
			startDatabase(network),
			startFirebase(network),
			startElasticsearch(network),
		]);

		sql = postgres(pg.getConnectionUri());
		motiva = await startMotiva(network);

		api = await startApi(network, process.env.LICENSE_KEY ?? "");
		worker = await startWorker(network, process.env.LICENSE_KEY ?? "");

		vars = await performInitialSetup(sql, uri(network, api, 8080));
	},
	15 * 60 * 1000,
);

afterAll(async () => {
	await sql?.end();
	await worker?.stop();
	await api?.stop();
	await motiva?.stop();
	await s3?.stop();
	await es?.stop();
	await pg?.stop();
	await fb?.stop();
});

describe("Initial setup", () => {
	it("responds to liveness", async () => {
		let health = await fetch(`${uri(network, api, 8080)}/liveness`);

		expect(health.status).toBe(200);
	});

	it("perform transaction monitoring screening check", async () => {
		await testTransactionMonitoring(uri(network, api, 8080), vars.scenarioId);
	});

	it("perform outgoing continuous screening on ingestion", async () => {
		await testOutgoingContinuousMonitoring(
			sql,
			uri(network, api, 8080),
			vars.continuousScreeningConfigId,
		);
	});

	it(
		"perform incoming continuous screening on dataset update",
		async () => {
			await testIncomingContinuousMonitoring(
				network,
				sql,
				s3,
				motiva,
				uri(network, api, 8080),
				vars.continuousScreeningConfigId,
			);
		},
		60 * 1000,
	);
});
