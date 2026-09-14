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

		const startupResults = await Promise.allSettled([
			buildMarble(),
			startDatabase(network).then((container) => (pg = container)),
			startFirebase(network).then((container) => (fb = container)),
			startElasticsearch(network).then((container) => (es = container)),
		]);
		const startupFailure = startupResults.find(
			(result): result is PromiseRejectedResult => result.status === "rejected",
		);
		if (startupFailure) {
			throw startupFailure.reason;
		}

		sql = postgres(pg.getConnectionUri());
		motiva = await startMotiva(network);

		api = await startApi(network, process.env.LICENSE_KEY ?? "");
		worker = await startWorker(network, process.env.LICENSE_KEY ?? "");

		vars = await performInitialSetup(sql, uri(network, api, 8080));
	},
	15 * 60 * 1000,
);

afterAll(async () => {
	const cleanupResults = await Promise.allSettled([
		sql?.end(),
		worker?.stop(),
		api?.stop(),
		motiva?.stop(),
		s3?.stop(),
		es?.stop(),
		pg?.stop(),
		fb?.stop(),
	]);

	const networkResults = network
		? await Promise.allSettled([network.stop()])
		: [];
	const cleanupFailure = [...cleanupResults, ...networkResults].find(
		(result): result is PromiseRejectedResult => result.status === "rejected",
	);
	if (cleanupFailure) {
		throw cleanupFailure.reason;
	}
});

describe("Initial setup", () => {
	it("responds to liveness", async () => {
		let health = await fetch(`${uri(network, api, 8080)}/liveness`);

		expect(health.status).toBe(200);
	});

	it("perform transaction monitoring screening check", async () => {
		await testTransactionMonitoring(uri(network, api, 8080), vars.scenarioId);
	});

	it(
		"perform outgoing continuous screening on ingestion",
		async () => {
			await testOutgoingContinuousMonitoring(
				sql,
				uri(network, api, 8080),
				vars.continuousScreeningConfigId,
			);
		},
		30 * 1000,
	);

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
		300 * 1000,
	);
});
