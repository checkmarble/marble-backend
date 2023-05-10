import {
  onAuthStateChanged,
  type User as FirebaseUser,
  signInWithPopup,
} from "firebase/auth";
import {
  SignInError,
  type OnAuthenticatedUserChanged,
  AuthenticatedUser,
} from "@/models";
import { FirebaseWrapper } from "@/infra/firebase";

export class AuthenticationRepository {
  firebase: FirebaseWrapper;

  constructor(firebase: FirebaseWrapper) {
    this.firebase = firebase;
  }

  // Note: onAuthenticatedUserChanged returns the unsubcribe function.
  onAuthenticatedUserChanged(
    onAuthenticatedUserChanged: OnAuthenticatedUserChanged
  ): () => void {
    return onAuthStateChanged(
      this.firebase.auth,
      (user: FirebaseUser | null) => {
        if (user) {
          onAuthenticatedUserChanged(adaptAuthenticatedUser(user));
        } else {
          onAuthenticatedUserChanged(null);
        }
      }
    );
  }

  get currentUser(): AuthenticatedUser | null {
    const user = this.firebase.auth.currentUser;
    return user ? adaptAuthenticatedUser(user) : null;
  }

  async fetchIdToken(forceRefresh = false): Promise<string> {
    const user = this.firebase.auth.currentUser;
    if (!user) {
      throw Error("No authenticated user, no token");
    }
    return await user.getIdToken(forceRefresh);
  }

  async signIn() {
    // source: https://firebase.google.com/docs/auth/web/google-signin
    try {
      const result = await signInWithPopup(
        this.firebase.auth,
        this.firebase.googleAuthProvider
      );
      // // This gives you a Google Access Token. You can use it to access the Google API.
      // const credential = GoogleAuthProvider.credentialFromResult(result);
      // const token = credential.accessToken;

      // // The signed-in user info.
      console.log(
        `User ${result.user.email} signed in with ${result.providerId} using (${result.operationType})`
      );

      // // IdP data available using getAdditionalUserInfo(result)
      // // ...
    } catch (error) {
      // // Handle Errors here.
      // const errorCode = error.code;
      // const errorMessage = error.message;
      // // The email of the user's account used.
      // const email = error.customData.email;
      // The AuthCredential type that was used.
      // const credential = GoogleAuthProvider.credentialFromError(error);
      // ...
      if (error instanceof Error) {
        throw new SignInError(`Sign in error`, error);
      } else {
        throw error;
      }
    }
  }

  async signOut() {
    await this.firebase.auth.signOut();
  }
}

function adaptAuthenticatedUser(user: FirebaseUser): AuthenticatedUser {
  return {
    uid: user.uid,
    email: user.email,
    displayName: user.displayName,
    photoURL: user.photoURL,
  };
}
