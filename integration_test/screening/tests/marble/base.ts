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
import { NATIVE_ARCH } from "./utils";
import path from "path";

export const startFirebase = async (
	network: StartedNetwork,
): Promise<StartedTestContainer> => {
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
	const pg = new PostgreSqlContainer("postgis/postgis:18-3.6-alpine")
		.withNetwork(network)
		.withNetworkAliases("db")
		.withUsername("postgres")
		.withPassword("marble")
		.withDatabase("marble");

	return pg.start();
};

export const startApi = async (
	network: StartedNetwork,
	licenseKey: string,
): Promise<StartedTestContainer> => {
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
			LICENSE_KEY: licenseKey,
			KILL_IF_READ_LICENSE_ERROR: "1",
			ENV: "production",
			PORT: "8080",
			PG_CONNECTION_STRING:
				"postgres://postgres:marble@db:5432/marble?sslmode=disable",
			CREATE_ORG_NAME: "Zorg",
			CREATE_ORG_ADMIN_EMAIL: "jbe@zorg.com",
			FIREBASE_AUTH_EMULATOR_HOST: "firebase:9099",
			FIREBASE_PROJECT_ID: "test-project",
			FIREBASE_API_KEY: "dummy",
			SCREENING_OPENSANCTIONS_API_HOST: "http://motiva:8000",
		})
		.withCommand([
			"sh",
			"-c",
			"apt update && apt install -y ca-certificates libgeos++-dev && /marble-backend --migrations --server",
		]);

	return api.start();
};
