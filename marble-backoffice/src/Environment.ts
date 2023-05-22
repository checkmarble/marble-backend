export interface Environment {
  authEmulator: boolean;
  marbleBackend: URL;
}

export const Environments: Record<string, Environment> = {
  Local: {
    authEmulator: true,
    marbleBackend: new URL("http://localhost:8080"),
  },

  Staging: {
    authEmulator: false,
    marbleBackend: new URL("https://marble-backend-gsmyteqtsa-od.a.run.app"),
  },
};
