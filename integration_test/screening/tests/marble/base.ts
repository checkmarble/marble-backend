import {
	GenericContainer,
	getContainerRuntimeClient,
	ImageName,
	StartedNetwork,
	StartedTestContainer,
	Wait,
} from "testcontainers";
import {
	PostgreSqlContainer,
	StartedPostgreSqlContainer,
} from "@testcontainers/postgresql";
import { NATIVE_ARCH, uri } from "./utils";
import path from "path";
import { CreateBucketCommand, S3Client } from "@aws-sdk/client-s3";

export const startFirebase = async (
	network: StartedNetwork,
): Promise<StartedTestContainer> => {
	console.log("starting firebase...");

	const firebase = new GenericContainer(
		"europe-west1-docker.pkg.dev/marble-infra/marble/firebase-emulator:latest",
	)
		.withNetwork(network)
		.withNetworkAliases("firebase")
		.withExposedPorts(9099);

	return firebase.start();
};

export const startDatabase = async (
	network: StartedNetwork,
): Promise<StartedPostgreSqlContainer> => {
	console.log("starting database...");

	const pg = new PostgreSqlContainer("postgis/postgis:18-3.6-alpine")
		.withNetwork(network)
		.withNetworkAliases("db")
		.withUsername("postgres")
		.withPassword("marble")
		.withDatabase("marble")
		.withCommand(["-c", "wal_level=logical"]);

	return pg.start();
};

export const startS3 = async (
	network: StartedNetwork,
): Promise<StartedTestContainer> => {
	console.log("starting s3...");

	const s3 = new PostgreSqlContainer("ghcr.io/versity/versitygw:v1.6.0")
		.withNetwork(network)
		.withNetworkAliases("s3")
		.withExposedPorts(7070)
		.withWaitStrategy(Wait.forHttp("/health", 7070))
		.withEnvironment({
			VGW_BACKEND: "posix",
			VGW_HEALTH: "/health",
			ROOT_ACCESS_KEY: "root",
			ROOT_SECRET_KEY: "azertyuiop",
		})
		.withCommand(["posix", "/tmp"]);

	const c = await s3.start();

	const client = new S3Client({
		endpoint: uri(network, c, 7070),
		region: "us-east-1",
		credentials: { accessKeyId: "root", secretAccessKey: "azertyuiop" },
	});

	await client.send(new CreateBucketCommand({ Bucket: "marble" }));

	return c;
};

const COMMON_ENV = {
	KILL_IF_READ_LICENSE_ERROR: "1",
	ENV: "production",
	MARBLE_API_URL: "http://api:8080",
	INGESTION_BUCKET_URL:
		"s3://marble?endpoint=http://s3:7070&region=us-east-1&use_path_style=true&disable_https=true",
	CONTINUOUS_SCREENING_BUCKET_URL:
		"s3://marble?endpoint=http://s3:7070&region=us-east-1&use_path_style=true&disable_https=true",
	SCREENING_OPENSANCTIONS_API_HOST: "http://motiva:8000",
	AWS_ACCESS_KEY_ID: "root",
	AWS_SECRET_ACCESS_KEY: "azertyuiop",
	SCREENING_INDEXER_TOKEN: "authtoken",
};

export const startApi = async (
	network: StartedNetwork,
	licenseKey: string,
): Promise<StartedTestContainer> => {
	console.log("starting api...");

	const client = await getContainerRuntimeClient();

	await client.image.pull(ImageName.fromString("debian:trixie"), {
		force: true,
		platform: NATIVE_ARCH,
	});

	const api = new GenericContainer("debian:trixie")
		.withPlatform(NATIVE_ARCH)
		.withNetwork(network)
		.withNetworkAliases("api")
		.withExposedPorts(8080)
		.withWaitStrategy(Wait.forHttp("/liveness", 8080))
		.withCopyFilesToContainer([
			{
				source: path.resolve("../../marble-backend"),
				target: "/marble-backend",
				mode: 666,
			},
		])
		.withEnvironment({
			...COMMON_ENV,
			LICENSE_KEY: licenseKey,
			PORT: "8080",
			PG_CONNECTION_STRING:
				"postgres://postgres:marble@db:5432/marble?sslmode=disable",
			CREATE_ORG_NAME: "Zorg",
			CREATE_ORG_ADMIN_EMAIL: "jbe@zorg.com",
			FIREBASE_AUTH_EMULATOR_HOST: "firebase:9099",
			FIREBASE_PROJECT_ID: "test-project",
			FIREBASE_API_KEY: "dummy",
		})
		.withCommand([
			"sh",
			"-c",
			"apt update && apt install -y ca-certificates libgeos++-dev && /marble-backend --migrations --server",
		]);

	return api.start();
};

export const startWorker = async (
	network: StartedNetwork,
	licenseKey: string,
): Promise<StartedTestContainer> => {
	console.log("starting worker...");

	const client = await getContainerRuntimeClient();

	await client.image.pull(ImageName.fromString("debian:trixie"), {
		force: true,
		platform: NATIVE_ARCH,
	});

	const worker = new GenericContainer("debian:trixie")
		.withPlatform(NATIVE_ARCH)
		.withNetwork(network)
		.withNetworkAliases("worker")
		.withExposedPorts(9191)
		.withWaitStrategy(Wait.forHttp("/liveness", 9191))
		.withCopyFilesToContainer([
			{
				source: path.resolve("../../marble-backend"),
				target: "/marble-backend",
				mode: 666,
			},
		])
		.withEnvironment({
			...COMMON_ENV,
			CLOUD_RUN_PROBE_PORT: "9191",
			LICENSE_KEY: licenseKey,
			PG_CONNECTION_STRING:
				"postgres://postgres:marble@db:5432/marble?sslmode=disable",
			SCAN_DATASET_UPDATES_INTERVAL: "5s",
			CREATE_FULL_DATASET_INTERVAL: "5s",
		})
		.withCommand([
			"sh",
			"-c",
			"apt update && apt install -y ca-certificates libgeos++-dev && /marble-backend --worker",
		]);

	return worker.start();
};
