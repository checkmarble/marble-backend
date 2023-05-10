import { FirebaseWrapper } from "@/infra/firebase";
import { AuthenticationRepository } from "./AuthenticationRepository";

export interface Repositories {
  authenticationRepository: AuthenticationRepository;
}

export function makeRepositories(firebase: FirebaseWrapper): Repositories {
  return {
    authenticationRepository: new AuthenticationRepository(firebase),
  };
}
