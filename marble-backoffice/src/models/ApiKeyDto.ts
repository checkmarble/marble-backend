import * as yup from "yup";
import { adaptRole } from "./Role";
import { ApiKey } from "./ApiKey";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

const ApiKeySchema = yup.object({
  organization_id: yup.string().defined(),
  key: yup.string().required(),
  role: yup.string().required(),
});

export type ApiKeyDto = yup.InferType<typeof ApiKeySchema>;

export function adaptApiKey(dto: ApiKeyDto): ApiKey {
  return {
    organizationId: dto.organization_id,
    role: adaptRole(dto.role),
    key: dto.key,
  };
}

export function adaptApiKeysApiResult(json: unknown) : ApiKey[] {
  return adaptDtoWithYup(
    json,
    yup.object({
      api_keys: yup.array().of(ApiKeySchema).required(),
    })
  ).api_keys.map((dto) => adaptApiKey(dto));
}
