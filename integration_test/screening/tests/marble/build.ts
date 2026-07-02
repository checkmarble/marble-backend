import fs from "fs";

import path from "path";
import {
	GenericContainer,
	getContainerRuntimeClient,
	ImageName,
	Wait,
} from "testcontainers";
import { NATIVE_ARCH } from "./utils";

const BUILDER_IMAGE = "docker.io/golang:1.26.2";

export const buildMarble = async () => {
	if (fs.existsSync("../../marble-backend")) {
		return;
	}

	const client = await getContainerRuntimeClient();

	await client.image.pull(ImageName.fromString(BUILDER_IMAGE), {
		force: true,
		platform: NATIVE_ARCH,
	});

	const builder = new GenericContainer(BUILDER_IMAGE)
		.withPlatform(NATIVE_ARCH)
		.withEnvironment({ CGO_ENABLED: "1" })
		.withWaitStrategy(Wait.forOneShotStartup())
		.withStartupTimeout(5 * 60 * 1000)
		.withBindMounts([{ source: path.resolve("../.."), target: "/app" }])
		.withWorkingDir("/app")
		.withCommand([
			"sh",
			"-c",
			"apt update && apt install -y libgeos++-dev && go build -buildvcs=false -ldflags '-s -w' .",
		]);

	await builder.start();
};
