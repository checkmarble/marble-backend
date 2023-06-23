import { type MarbleApi } from "@/infra/MarbleApi";
import type { Scenario, AstNode } from "@/models";
import { adaptScenariosApiResult } from "@/models/ScenarioDto";
import { adapAstValidateSchemaResult } from "@/models/AstExpressionDto";

export interface ScenariosRepository {
  marbleApi: MarbleApi;
}

export async function fetchScenarios(
  repository: ScenariosRepository,
  organizationId: string
): Promise<Scenario[]> {
  return adaptScenariosApiResult(
    await repository.marbleApi.scenariosOfOrganization(organizationId)
  );
}

export async function validateAstExpression(
  repository: ScenariosRepository,
  // organizationId: string,
  expression: AstNode
) {
  return adapAstValidateSchemaResult(
    await repository.marbleApi.validateAstExpression(expression)
  );
}
