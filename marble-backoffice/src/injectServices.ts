import { type Repositories } from "./repositories";
import { AuthenticationService } from "./services";
import type {
  OrganizationService,
  UserService,
  ApiKeyService,
  AstExpressionService,
  // AstEditorService,
  ScenarioService,
  DataModelService,
  DemoScenarioService,
  ScheduledExecutionService,
} from "./services";

export interface Services {
  authenticationService: AuthenticationService;
  organizationService: OrganizationService;
  scenarioService: ScenarioService;
  dataModelService: DataModelService;
  demoScenarioService: DemoScenarioService;
  userService: UserService;
  apiKeyService: ApiKeyService;
  astExpressionService: AstExpressionService;
  // astEditorService: AstEditorService;
  scheduledExecutionService: ScheduledExecutionService;
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
      userRepository: repositories.userRepository,
    },
    scenarioService: {
      scenariosRepository: repositories.scenarioRepository,
    },
    dataModelService: {
      dataModelRepository: repositories.dataModelRepository,
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
    // astEditorService: {
    //   scenariosRepository: repositories.scenarioRepository,
    // },
    demoScenarioService: {
      organizationRepository: repositories.organizationRepository,
      scenariosRepository: repositories.scenarioRepository,
    },
    scheduledExecutionService: {
      scheduledExecutionRepository: repositories.organizationRepository,
    },
  };
}

export default function services(): Services {
  if (globalServices === null) {
    throw Error("Global services not initialized.");
  }
  return globalServices;
}
