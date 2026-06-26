import { StartedPostgreSqlContainer } from "@testcontainers/postgresql";
import { startFirebase, startApi, startDatabase } from "./marble/base";
import { Network, StartedNetwork, StartedTestContainer } from "testcontainers";
import { performInitialSetup, type SetupOutput } from "./marble/setup";
import postgres, { Sql } from "postgres";
import { startElasticsearch, startMotiva } from "./marble/screening";
import { testTransactionMonitoring } from "./transaction";
import { buildMarble } from "./marble/build";
import { uri } from "./marble/utils";

var network: StartedNetwork;

var fb: StartedTestContainer;
var pg: StartedPostgreSqlContainer;
var es: StartedTestContainer;
var motiva: StartedTestContainer;
var api: StartedTestContainer;
var dbConn: Sql;

var vars: SetupOutput;

beforeAll(
	async () => {
		network = await new Network().start();

		[, pg, fb, es] = await Promise.all([
			buildMarble(),
			startDatabase(network),
			startFirebase(network),
			startElasticsearch(network),
		]);

		dbConn = postgres(pg.getConnectionUri());
		motiva = await startMotiva(network);
		api = await startApi(network, process.env.LICENSE_KEY ?? "");

		vars = await performInitialSetup(dbConn, uri(network, api, 8080));
	},
	15 * 60 * 1000,
);

afterAll(async () => {
	await dbConn?.end();
	await api?.stop();
	await motiva?.stop();
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
});
