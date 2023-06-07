import { FirebaseWrapper } from "@/infra/firebase";
import { AuthenticationRepository } from "./AuthenticationRepository";
import { OrganizationRepository } from "./OrganizationRepository";
import { UserRepository } from "./UserRepository";
import { ScenariosRepository } from "./ScenariosRepository";
import { MarbleApi } from "@/infra/MarbleApi";
import { MarbleApiFetcher } from "@/infra/MarbleApiFetcher";
import { setAuthorizationBearerHeader } from "@/infra/fetchUtils";

export interface Repositories {
  authenticationRepository: AuthenticationRepository;
  organizationRepository: OrganizationRepository;
  userRepository: UserRepository;
  scenarioRepository: ScenariosRepository;
}

export function makeRepositories(
  firebase: FirebaseWrapper,
  backendUrl: URL
): Repositories {
  const authenticationRepository = new AuthenticationRepository(firebase);

  const fetcher = new MarbleApiFetcher(
    backendUrl,
    async (headers: Headers) => {
      const idToken = await authenticationRepository.fetchIdToken()
      setAuthorizationBearerHeader(headers, idToken);
    }
  )
  const marbleApi = new MarbleApi(backendUrl, fetcher);

  return {
    authenticationRepository: authenticationRepository,
    organizationRepository: { marbleApi },
    userRepository: { marbleApi },
    scenarioRepository: { marbleApi },
  };
}
