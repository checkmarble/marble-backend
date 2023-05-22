import {
  type FirebaseApp,
  initializeApp,
} from "firebase/app";
import {
  getAuth,
  connectAuthEmulator,
  type Auth,
  GoogleAuthProvider,
} from "firebase/auth";

export interface FirebaseWrapper {
  app: FirebaseApp;
  auth: Auth;
  googleAuthProvider: GoogleAuthProvider;
}

export function initializeFirebase(authEmulator: boolean): FirebaseWrapper {
  // Initialize Firebase
  const app = initializeApp({
    apiKey: "AIzaSyAElc2shIKIrYzLSzWmWaZ1C7yEuoS-bBw",
    authDomain: "tokyo-country-381508.firebaseapp.com",
    projectId: "tokyo-country-381508",
    storageBucket: "tokyo-country-381508.appspot.com",
    messagingSenderId: "1047691849054",
    appId: "1:1047691849054:web:a5b69dd2ac584c1160b3cf",
  });

  const auth = getAuth(app);
  if (authEmulator) {
    connectAuthEmulator(auth, "http://localhost:9099");
  }

  // To apply the default browser preference instead of explicitly setting it.
  auth.useDeviceLanguage();

  const googleAuthProvider = new GoogleAuthProvider();

  return {
    app,
    auth,
    googleAuthProvider,
  };
}
