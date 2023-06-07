import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

// ------ ApiKeyDto

const ApiKeySchema = yup.object({
  organization_id: yup.string().defined(),
  key: yup.string().required(),
  role: yup.string().required(),
});

export type ApiKeyDto = yup.InferType<typeof ApiKeySchema>;

// ------ ApiKeysApiResult

const ApiKeysApiResultSchema = yup.object({
    api_keys: yup.array().of(ApiKeySchema).required(),
});

export type ApiKeysApiResultDto = yup.InferType<
  typeof ApiKeysApiResultSchema
>;

export function adaptApiKeysResultDto(
  json: unknown
): ApiKeysApiResultDto {
  return adaptDtoWithYup(json, ApiKeysApiResultSchema);
}
