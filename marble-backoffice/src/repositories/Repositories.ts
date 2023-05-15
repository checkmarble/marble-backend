import { FirebaseWrapper } from "@/infra/firebase";
import { AuthenticationRepository } from "./AuthenticationRepository";
import { OrganizationRepository } from "./OrganizationRepository";
import { MarbleApi } from "@/infra/MarbleApi";

export interface Repositories {
  authenticationRepository: AuthenticationRepository;
  organizationRepository: OrganizationRepository;
}

export function makeRepositories(
  firebase: FirebaseWrapper,
  backendUrl: URL
): Repositories {
  const authenticationRepository = new AuthenticationRepository(firebase);
  const marbleApi = new MarbleApi(backendUrl, () =>
    authenticationRepository.fetchIdToken()
  );

  return {
    authenticationRepository: authenticationRepository,
    organizationRepository: { marbleApi },
  };
}
