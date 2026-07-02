import { StartedNetwork, StartedTestContainer } from "testcontainers";

export const NATIVE_ARCH = `linux/${process.arch == "x64" ? "x86_64" : process.arch}`;
export const X86_64 = "linux/x86_64";

export const uri = (
	network: StartedNetwork,
	container: StartedTestContainer,
	port: number,
): string => {
	const host = container.getIpAddress(network.getName());

	return `http://${host}:${port}`;
};
