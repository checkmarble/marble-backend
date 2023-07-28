import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import { type Scenario } from "./Scenario";

const ScenarioSchema = yup.object({
  id: yup.string().required(),
  organization_id: yup.string().required(),
  name: yup.string().required(),
  description: yup.string().required(),
  triggerObjectType: yup.string().required(),
  createdAt: yup.date().required(),
  liveVersionId: yup.string().nullable(),
});

export type ScenarioDto = yup.InferType<typeof ScenarioSchema>;

export function adaptScenario(dto: ScenarioDto): Scenario {
  return {
    scenarioId: dto.id,
    organizationId: dto.organization_id,
    name: dto.name,
    description: dto.description,
    triggerObjectType: dto.triggerObjectType,
    createdAt: dto.createdAt,
    liveVersionId: dto.liveVersionId === undefined ? null : dto.liveVersionId,
    liveIteration: null,
  };
}

export function adaptScenariosApiResult(json: unknown): Scenario[] {
  const dtos = adaptDtoWithYup(json, yup.array().required().of(ScenarioSchema));
  return dtos.map(adaptScenario);
}

export function adaptSingleScenarioApiResult(json: unknown): Scenario {
  const dto = adaptDtoWithYup(json, ScenarioSchema);
  return adaptScenario(dto);
}
