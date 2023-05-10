import { type PropsWithChildren } from "react";
import services from "@/injectServices";
import { useAuthentication, AuthenticatedUserContext } from "@/services";
import Login from "./Login";

function AuthFence(props: PropsWithChildren): JSX.Element {
  const { authenticatedUser } = useAuthentication(
    services().authenticationService
  );

  return (
    <AuthenticatedUserContext.Provider value={authenticatedUser}>
      {authenticatedUser ? props.children : <Login />}
    </AuthenticatedUserContext.Provider>
  );
}

export default AuthFence;
