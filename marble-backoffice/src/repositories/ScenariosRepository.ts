import { MarbleApi } from "@/infra/MarbleApi";
import { Scenarios, adaptScenario } from "@/models/Scenarios";
import { adaptScenariosApiResultDto } from "@/models/ScenariosDto";

export interface ScenariosRepository {
  marbleApi: MarbleApi;
}

export async function fetchScenarios(
  repository: ScenariosRepository,
  organizationId: string
): Promise<Scenarios[]> {
  const result = adaptScenariosApiResultDto(
    await repository.marbleApi.scenariosOfOrganization(organizationId)
  );
  return result.map(adaptScenario);
}
