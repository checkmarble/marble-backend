import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import { adaptRole } from "./Role";
import { Credentials } from "./Credentials";

const CredentialsSchema = yup.object({
  organization_id: yup.string().defined(),
  role: yup.string().required(),
  actor_identity: yup
    .object({
      user_id: yup.string(),
      email: yup.string(),
      api_key_name: yup.string(),
    })
    .required(),
});

export function adaptCredential(json: unknown): Credentials {

  const dto = adaptDtoWithYup(
    json,
    yup.object({
      credentials: CredentialsSchema,
    })
  ).credentials

  return {
    organizationId: dto.organization_id,
    role: adaptRole(dto.role),
    actorIdentity: {
        userId: dto.actor_identity.user_id || "",
        email: dto.actor_identity.email || "",
        apiKeyName: dto.actor_identity.api_key_name || "",
    },
  };
}
