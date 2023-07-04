import { type Repositories } from "./repositories";
import { AuthenticationService } from "./services";
import type {
  OrganizationService,
  UserService,
  ApiKeyService,
  AstExpressionService,
} from "./services";

export interface Services {
  authenticationService: AuthenticationService;
  organizationService: OrganizationService;
  userService: UserService;
  apiKeyService: ApiKeyService;
  astExpressionService: AstExpressionService;
}

let globalServices: Services | null;

export function initializeServices(repositories: Repositories) {
  if (globalServices) {
    throw Error("Global services already initialized");
  }
  globalServices = {
    authenticationService: new AuthenticationService(
      repositories.authenticationRepository
    ),
    organizationService: {
      organizationRepository: repositories.organizationRepository,
      scenariosRepository: repositories.scenarioRepository,
      userRepository: repositories.userRepository,
    },
    userService: {
      userRepository: repositories.userRepository,
    },
    apiKeyService: {
      organizationRepository: repositories.organizationRepository,
      marbleApiWithApiKeyFactory: repositories.marbleApiWithApiKeyFactory,
    },
    astExpressionService: {
      scenarioRepository: repositories.scenarioRepository,
    },
  };
}

export default function services(): Services {
  if (globalServices === null) {
    throw Error("Global services not initialized.");
  }
  return globalServices;
}
