import { type MarbleApi } from "@/infra/MarbleApi";
import type { Scenario, AstNode, EditorIdentifiers } from "@/models";
import {
  adaptScenariosApiResult,
  adaptSingleScenarioApiResult,
} from "@/models/ScenarioDto";
import {
  adapAstValidateSchemaResult,
  adaptAstNodeDto,
} from "@/models/AstExpressionDto";
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

export async function validateAstExpression(
  repository: ScenariosRepository,
  organizationId: string,
  expression: AstNode
) {
  return adapAstValidateSchemaResult(
    await repository.marbleApi.validateAstExpression(
      organizationId,
      adaptAstNodeDto(expression)
    )
  );
}

export async function runAstExpression(
  repository: ScenariosRepository,
  organizationId: string,
  expression: AstNode
) {
  return await repository.marbleApi.runAstExpression(
    organizationId,
    adaptAstNodeDto(expression)
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
