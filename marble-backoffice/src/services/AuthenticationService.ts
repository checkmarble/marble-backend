import { useContext, createContext, useEffect, useState } from "react";
import { AuthenticationRepository } from "@/repositories";
import { type AuthenticatedUser } from "@/models";

export class AuthenticationService {
  authenticationRepository: AuthenticationRepository;

  constructor(authenticationRepository: AuthenticationRepository) {
    this.authenticationRepository = authenticationRepository;
  }

  async signOut() {
    await this.authenticationRepository.signOut();
  }
}

export const AuthenticatedUserContext = createContext<AuthenticatedUser | null>(
  null
);

export function useAuthentication(authService: AuthenticationService) {
  const [authenticatedUser, setAuthenticatedUser] =
    useState<AuthenticatedUser | null>(null);

  // bind firebase callback once
  useEffect(() => {
    const unsubscribe =
      authService.authenticationRepository.onAuthenticatedUserChanged(
        (user) => {
          setAuthenticatedUser(user);
        }
      );

    return unsubscribe;
  }, [authService]);

  return {
    authenticatedUser,
  };
}

export function useAuthenticatedUser() {
  const user = useContext(AuthenticatedUserContext);
  if (!user) {
    throw Error(
      "User is supposed to be connected. Note that useAuthenticatedUser must be call in the children of AuthFence."
    );
  }

  return user;
}
