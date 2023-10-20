import { type FirebaseOptions } from "firebase/app";

export interface Environment {
  authEmulator: boolean;
  marbleBackend: URL;
  firebaseOptions: FirebaseOptions;
}

export const Environments: Record<string, Environment> = {
  development: {
    authEmulator: true,
    marbleBackend: new URL("http://localhost:8080"),
    firebaseOptions: {
      apiKey: "AIzaSyAElc2shIKIrYzLSzWmWaZ1C7yEuoS-bBw",
      authDomain: "tokyo-country-381508.firebaseapp.com",
      projectId: "tokyo-country-381508",
      storageBucket: "tokyo-country-381508.appspot.com",
      messagingSenderId: "1047691849054",
      appId: "1:1047691849054:web:59e5df4b6dbdacbe60b3cf",
    },
  },

  staging: {
    authEmulator: false,
    marbleBackend: new URL("https://api.staging.checkmarble.com"),
    firebaseOptions: {
      apiKey: "AIzaSyAElc2shIKIrYzLSzWmWaZ1C7yEuoS-bBw",
      authDomain: "backoffice.staging.checkmarble.com",
      projectId: "tokyo-country-381508",
      storageBucket: "tokyo-country-381508.appspot.com",
      messagingSenderId: "1047691849054",
      appId: "1:1047691849054:web:59e5df4b6dbdacbe60b3cf",
    },
  },

  production: {
    authEmulator: false,
    marbleBackend: new URL("https://api.checkmarble.com"),
    firebaseOptions: {
      apiKey: "AIzaSyDxzrr5GLnlbVQfeSWjBK6_w85rACgXQrg",
      authDomain: "marble-backoffice-production.web.app",
      projectId: "marble-prod-1",
      storageBucket: "marble-prod-1.appspot.com",
      messagingSenderId: "280431296971",
      appId: "1:280431296971:web:ff089aa051073474f8f64e",
    },
  },

  test_terraform: {
    authEmulator: false,
    marbleBackend: new URL("https://marble-backend-ngbphj56ia-ew.a.run.app"),
    firebaseOptions: {
      apiKey: "AIzaSyDBX1gn8_ISZIe0MI2ZimE71zJN87T5fVc",
      authDomain: "marble-test-terraform.firebaseapp.com",
      projectId: "marble-test-terraform",
      storageBucket: "marble-test-terraform.appspot.com",
      messagingSenderId: "1055186671888",
      appId: "1:1055186671888:web:04ccd4d77997ddf1b5ad95",
    },
  },
};

export function buildEnvironment(): Environment {
  const environmentName = import.meta.env.MODE;
  const enviroment = Environments[environmentName];
  if (!enviroment) {
    throw Error(`Unknown environment ${environmentName}`);
  }
  console.log(`Using environment ${environmentName}`);
  return enviroment;
}
