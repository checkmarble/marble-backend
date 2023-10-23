import { type MarbleApi } from "@/infra/MarbleApi";
import type {
  Scenario,
  EditorIdentifiers,
  CreateScenario,
  UpdateRule,
  UpdateIteration,
  // AstNode,
} from "@/models";
import {
  adaptScenariosApiResult,
  adaptSingleScenarioApiResult,
} from "@/models/ScenarioDto";
import { adaptRuleApiResult, adaptUpdateRuleApiResult } from "@/models/RuleDto";
import {
  adaptIterationApiResult,
  adaptIterationWithValidationApiResult,
  adaptListIterationsApiResult,
  // adaptValidateIterationApiResult,
} from "@/models/IterationDto";
import { adaptEditorIdentifiers } from "@/models/EditorIdentifiersDto";

export interface ScenariosRepository {
  marbleApi: MarbleApi;
}

export async function fetchScenariosOfOrganization(
  repository: ScenariosRepository,
  organizationId: string
): Promise<Scenario[]> {
  return adaptScenariosApiResult(
    await repository.marbleApi.scenariosOfOrganization(organizationId)
  );
}

export async function fetchScenario(
  repository: ScenariosRepository,
  scenarioId: string
): Promise<Scenario> {
  return adaptSingleScenarioApiResult(
    await repository.marbleApi.scenariosById(scenarioId)
  );
}

export async function fetchEditorIdentifiers(
  repository: ScenariosRepository,
  scenarioId: string
): Promise<EditorIdentifiers> {
  return adaptEditorIdentifiers(
    await repository.marbleApi.editorIdentifiers(scenarioId)
  );
}

export async function postScenario(
  repository: ScenariosRepository,
  organizationId: string,
  createScenario: CreateScenario
) {
  return adaptSingleScenarioApiResult(
    await repository.marbleApi.postScenario(organizationId, createScenario)
  );
}

export async function fetchIteration(
  repository: ScenariosRepository,
  organizationId: string,
  iterationId: string
) {
  return adaptIterationApiResult(
    await repository.marbleApi.fetchIterationById(organizationId, iterationId)
  );
}

export async function fetchIterationsOfScenario(
  repository: ScenariosRepository,
  organizationId: string,
  scenarioId: string
) {
  return adaptListIterationsApiResult(
    await repository.marbleApi.listIterations(organizationId, scenarioId)
  );
}

export async function postIteration(
  repository: ScenariosRepository,
  organizationId: string,
  scenarioId: string
) {
  return adaptIterationApiResult(
    await repository.marbleApi.postIteration(organizationId, scenarioId)
  );
}

export async function patchIteration(
  repository: ScenariosRepository,
  organizationId: string,
  iterationId: string,
  changes: UpdateIteration
) {
  return adaptIterationWithValidationApiResult(
    await repository.marbleApi.patchIteration(
      organizationId,
      iterationId,
      changes
    )
  );
}

export async function postRule(
  repository: ScenariosRepository,
  organizationId: string,
  iterationId: string
) {
  return adaptRuleApiResult(
    await repository.marbleApi.postRule(organizationId, iterationId)
  );
}

export async function updateRule(
  repository: ScenariosRepository,
  organizationId: string,
  ruleId: string,
  changes: UpdateRule
) {
  return adaptUpdateRuleApiResult(
    await repository.marbleApi.patchRule(organizationId, ruleId, changes)
  );
}

// export async function validateIteration(
//   repository: ScenariosRepository,
//   iterationId: string
// ) {
//   return adaptValidateIterationApiResult(
//     await repository.marbleApi.validateIteration(iterationId)
//   );
// }

// export async function validateIterationWithGivenTriggerOrRule(
//   repository: ScenariosRepository,
//   iterationId: string,
//   triggerOrRule: AstNode,
//   ruleId: string | null
// ) {
//   return adaptValidateIterationApiResult(
//     await repository.marbleApi.validateScenarioIterationWithGivenTriggerOrRule(
//       iterationId,
//       triggerOrRule,
//       ruleId
//     )
//   );
// }

export async function publishIteration(
  repository: ScenariosRepository,
  organizationId: string,
  iterationId: string
) {
  await repository.marbleApi.postScenarioIterationPublication(
    organizationId,
    iterationId,
    "publish"
  );
}
