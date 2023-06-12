import { type MarbleApi } from "@/infra/MarbleApi";
import type { Scenario } from "@/models/Scenario";
import { adaptScenariosApiResult } from "@/models/ScenarioDto";

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
