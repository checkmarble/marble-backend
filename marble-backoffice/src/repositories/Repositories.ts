import { FirebaseWrapper } from "@/infra/firebase";
import { AuthenticationRepository } from "./AuthenticationRepository";
import { OrganizationRepository } from "./OrganizationRepository";
import { UserRepository } from "./UserRepository";
import { ScenariosRepository } from "./ScenariosRepository";
import { MarbleApi } from "@/infra/MarbleApi";
import { MarbleApiFetcher } from "@/infra/MarbleApiFetcher";
import { setAuthorizationBearerHeader } from "@/infra/fetchUtils";
import { DataModelRepository } from "./DataModelRepository";

export interface Repositories {
  authenticationRepository: AuthenticationRepository;
  organizationRepository: OrganizationRepository;
  userRepository: UserRepository;
  scenarioRepository: ScenariosRepository;
  dataModelRepository: DataModelRepository;
  marbleApiWithApiKeyFactory: (apiKey: string) => MarbleApi;
}

export function makeRepositories(
  firebase: FirebaseWrapper,
  backendUrl: URL
): Repositories {
  const authenticationRepository = new AuthenticationRepository(firebase);

  const marbleApiWithFirebaseToken = new MarbleApi(
    backendUrl,
    new MarbleApiFetcher(backendUrl, async (headers: Headers) => {
      const idToken = await authenticationRepository.fetchIdToken();
      setAuthorizationBearerHeader(headers, idToken);
    })
  );

  const marbleApiWithApiKeyFactory = (apiKey: string): MarbleApi => {
    return new MarbleApi(
      backendUrl,
      new MarbleApiFetcher(backendUrl, async (headers: Headers) => {
        headers.set("X-API-Key", apiKey);
      })
    );
  };

  return {
    authenticationRepository: authenticationRepository,
    organizationRepository: { marbleApi: marbleApiWithFirebaseToken },
    userRepository: { marbleApi: marbleApiWithFirebaseToken },
    scenarioRepository: { marbleApi: marbleApiWithFirebaseToken },
    dataModelRepository: { marbleApi: marbleApiWithFirebaseToken },
    marbleApiWithApiKeyFactory,
  };
}
