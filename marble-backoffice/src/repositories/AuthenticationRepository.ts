import {
  onAuthStateChanged,
  type User as FirebaseUser,
  signInWithRedirect,
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

      await signInWithRedirect(
        this.firebase.auth,
        this.firebase.googleAuthProvider
      );

    } catch (error) {
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
