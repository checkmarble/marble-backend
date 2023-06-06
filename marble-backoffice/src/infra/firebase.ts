import { type FirebaseApp, initializeApp } from "firebase/app";
import {
  getAuth,
  connectAuthEmulator,
  type Auth,
  GoogleAuthProvider,
  getRedirectResult,
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
    appId: "1:1047691849054:web:59e5df4b6dbdacbe60b3cf",
  });

  const auth = getAuth(app);
  if (authEmulator) {
    connectAuthEmulator(auth, "http://localhost:9099");
  }

  // To apply the default browser preference instead of explicitly setting it.
  auth.useDeviceLanguage();

  const googleAuthProvider = new GoogleAuthProvider();

  getRedirectResult(auth).then((userCredential) => {
      if (userCredential === null) {
        return
      }

      // The signed-in user info.
      console.log(
        `User ${userCredential.user.email} credentials from ${userCredential.providerId} with ${userCredential.operationType}`
      );
  }).catch((error : unknown) => {
    if (error instanceof Error) {
      // Handle Errors here.
      // const errorCode = error.code;
      // const errorMessage = error.message;
      // The email of the user's account used.
      // const email = error.customData.email;
      // The AuthCredential type that was used.
      // const credential = GoogleAuthProvider.credentialFromError(error);
    }
    throw error
  });

  return {
    app,
    auth,
    googleAuthProvider,
  };
}
