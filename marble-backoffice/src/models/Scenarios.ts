import { ScenarioDto } from "./ScenariosDto";

export interface Scenarios {
  scenariosId: string;
  name: string;
  description: string;
  triggerObjectType: string;
  createdAt: Date;
  liveVersionId: string | null;
}

export function adaptScenario(dto: ScenarioDto): Scenarios {
  return {
    scenariosId: dto.id,
    name: dto.name,
    description: dto.description,
    triggerObjectType: dto.triggerObjectType,
    createdAt: dto.createdAt,
    liveVersionId: dto.liveVersionId
  };
}
