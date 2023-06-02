import { type Repositories } from "./repositories";
import {
  AuthenticationService,
  type OrganizationService,
  type UserService,
} from "./services";

export interface Services {
  authenticationService: AuthenticationService;
  organizationService: OrganizationService;
  userService: UserService;
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
  };
}

export default function services(): Services {
  if (globalServices === null) {
    throw Error("Global services not initialized.");
  }
  return globalServices;
}
