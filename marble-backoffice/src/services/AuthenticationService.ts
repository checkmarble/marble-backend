import {
  useContext,
  createContext,
  useEffect,
  useState,
  useCallback,
} from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { AuthenticationRepository } from "@/repositories";
import {
  type AuthenticatedUser,
  type Authentication,
  AuthenticationStatus,
  PageLink,
  isRouteRequireAuthenticatedUser,
} from "@/models";

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
  const [authentication, setAuthentication] = useState<Authentication>({
    user: null,
    status: AuthenticationStatus.Unknown,
  });

  const navigator = useNavigate();
  const location = useLocation();

  useEffect(() => {
    // subscribe to authenticated user changes
    const unsubscribe =
      authService.authenticationRepository.onAuthenticatedUserChanged(
        (user) => {
          setAuthentication({
            user: user,
            status:
              user === null
                ? AuthenticationStatus.NotAuthenticated
                : AuthenticationStatus.Authenticated,
          });
        }
      );

    return unsubscribe;
  }, [authService]);

  const routeRequireAuthenticatedUser = isRouteRequireAuthenticatedUser(
    location.pathname
  );

  // redirect to login page when not authenticated
  useEffect(() => {
    if (
      routeRequireAuthenticatedUser &&
      authentication.status === AuthenticationStatus.NotAuthenticated
    ) {
      navigator(PageLink.loginWithRedirect(location.pathname));
    }
  }, [
    navigator,
    routeRequireAuthenticatedUser,
    authentication.status,
    location.pathname,
  ]);

  return {
    user: authentication.user,
    authLoading: authentication.status == AuthenticationStatus.Unknown,
    displayPrivatePage:
    routeRequireAuthenticatedUser &&
      authentication.status === AuthenticationStatus.Authenticated,
  };
}

export function useAuthenticatedUser(): AuthenticatedUser {
  const user = useContext(AuthenticatedUserContext);

  if (user === null) {
    throw Error(
      "User is supposed to be connected. Note that useAuthenticatedUser must be call in the children of AuthFence."
    );
  }
  return user;
}

export function useSignIn(
  authService: AuthenticationService,
  redirect: string | null
) {
  const navigator = useNavigate();
  const user = useContext(AuthenticatedUserContext);

  const signIn = useCallback(async () => {
    await authService.authenticationRepository.signIn();
  }, [authService]);

  useEffect(() => {
    if (user !== null) {
      navigator(redirect || "/");
    }
  }, [user, navigator, redirect]);

  return {
    signIn,
  };
}
