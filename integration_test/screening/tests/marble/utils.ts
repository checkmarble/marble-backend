import { StartedNetwork, StartedTestContainer } from "testcontainers";

export const NATIVE_ARCH = `linux/${process.arch == "x64" ? "x86_64" : process.arch}`;
export const X86_64 = "linux/x86_64";

export const uri = (
	network: StartedNetwork,
	container: StartedTestContainer,
	port: number,
): string => {
	return `http://localhost:${container.getMappedPort(port)}`;
};
