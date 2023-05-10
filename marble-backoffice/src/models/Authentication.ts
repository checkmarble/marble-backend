export interface AuthenticatedUser {
  uid: string;
  email: string | null;
  displayName: string | null;
  photoURL: string | null;
}

export type OnAuthenticatedUserChanged = (user: AuthenticatedUser | null) => void;
