import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

const ScenarioSchema = yup.object({
  id: yup.string().required(),
  name: yup.string().required(),
  description: yup.string().required(),
  triggerObjectType: yup.string().required(),
  createdAt: yup.date().required(),
  liveVersionId: yup.string().defined().nullable(),
});

export type ScenarioDto = yup.InferType<typeof ScenarioSchema>;

// ------ ScenariosApiResultDto

const ScenariosApiResultSchema = yup.array().required().of(ScenarioSchema);

export type ScenariosApiResultDto = yup.InferType<
  typeof ScenariosApiResultSchema
>;

export function adaptScenariosApiResultDto(
  json: unknown
): ScenariosApiResultDto {
  return adaptDtoWithYup(json, ScenariosApiResultSchema);
}
