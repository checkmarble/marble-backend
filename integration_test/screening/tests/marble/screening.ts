import {
	GenericContainer,
	StartedNetwork,
	StartedTestContainer,
	Wait,
} from "testcontainers";

const DEFAULT_MANIFEST = {
	catalogs: [
		{
			url: "https://data.opensanctions.org/datasets/latest/index.json",
			scope: "fr_assemblee",
			resource_name: "entities.ftm.json",
		},
	],
};

export const startElasticsearch = async (
	network: StartedNetwork,
): Promise<StartedTestContainer> => {
	console.log("starting elasticsearch...");

	const container = new GenericContainer("elasticsearch:9.4.2")
		.withNetwork(network)
		.withNetworkAliases("es")
		.withExposedPorts(9200)
		.withWaitStrategy(Wait.forHttp("/_cluster/health", 9200))
		.withEnvironment({
			"discovery.type": "single-node",
			"xpack.security.enabled": "false",
		});

	const es = await container.start();

	await triggerIndexing(network, DEFAULT_MANIFEST);

	return es;
};

export const triggerIndexing = async (
	network: StartedNetwork,
	manifest: any,
) => {
	console.log("starting indexer...");

	await new GenericContainer("ghcr.io/opensanctions/yente:5.3.0")
		.withNetwork(network)
		.withCopyContentToContainer([
			{
				content: JSON.stringify(manifest),
				target: "/manifest.json",
			},
		])
		.withEnvironment({
			YENTE_MANIFEST: "/manifest.json",
			YENTE_INDEX_URL: "http://es:9200",
		})
		.withWaitStrategy(Wait.forOneShotStartup())
		.withCommand(["yente", "reindex", "--force"])
		.start();
};

export const startMotiva = async (
	network: StartedNetwork,
): Promise<StartedTestContainer> => {
	console.log("starting motiva...");

	return new GenericContainer("ghcr.io/apognu/motiva:v0.9.2")
		.withPlatform("linux/x86_64")
		.withNetwork(network)
		.withNetworkAliases("motiva")
		.withExposedPorts(8000)
		.withWaitStrategy(Wait.forHttp("/healthz", 8000))
		.withCopyContentToContainer([
			{
				content: JSON.stringify(DEFAULT_MANIFEST),
				target: "/manifest.json",
			},
		])
		.withEnvironment({
			MANIFEST_URL: "/manifest.json",
			INDEX_URL: "http://es:9200",
		})
		.start();
};
