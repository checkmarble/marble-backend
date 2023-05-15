import { type Repositories } from "./repositories";
import { AuthenticationService, OrganizationService } from "./services";

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
    organizationService: new OrganizationService(
      repositories.organizationRepository
    ),
  };
}

export default function services(): Services {
  if (globalServices === null) {
    throw Error("Global services not initialized.");
  }
  return globalServices;
}
