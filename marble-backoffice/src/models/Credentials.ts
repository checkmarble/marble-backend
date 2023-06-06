import { type CredentialsDto } from "./CredentialsDto";
import { type Role, adaptRole } from "./Role";


export interface ActorIdentity {
    userId: string;
    email: string;
    apiKeyName: string;
  }

export interface Credentials {
  organizationId: string;
  role: Role;
  actorIdentity: ActorIdentity;
}

export function adaptCredential(dto: CredentialsDto): Credentials {
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

