import { type Repositories } from "./repositories";
import { AuthenticationService, type OrganizationService } from "./services";

export interface Services {
  authenticationService: AuthenticationService;
  organizationService: OrganizationService;
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
    },
  };
}

export default function services(): Services {
  if (globalServices === null) {
    throw Error("Global services not initialized.");
  }
  return globalServices;
}
