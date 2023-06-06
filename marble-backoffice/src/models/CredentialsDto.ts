import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

// ------ Credentials

const CredentialsSchema = yup.object({
  organization_id: yup.string().defined(),
  role: yup.string().required(),
  actor_identity: yup.object({
    user_id: yup.string(),
    email: yup.string(),
    api_key_name: yup.string(),
  }).required(),
});

export type CredentialsDto = yup.InferType<typeof CredentialsSchema>;

// ------ CredentialsApiResult

const CredentialsApiResultSchema = yup.object({
    credentials: CredentialsSchema,
});

export type CredentialsApiResultDto = yup.InferType<
  typeof CredentialsApiResultSchema
>;

export function adaptCredentialsApiResultDto(
  json: unknown
): CredentialsApiResultDto {
  return adaptDtoWithYup(json, CredentialsApiResultSchema);
}
