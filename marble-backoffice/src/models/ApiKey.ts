import { type ApiKeyDto } from "./ApiKeyDto";
import { type Role, adaptRole } from "./Role";

export interface ApiKey {
  organizationId: string;
  role: Role;
  key: string;
}

export function adaptApiKey(dto: ApiKeyDto): ApiKey {
  return {
    organizationId: dto.organization_id,
    role: adaptRole(dto.role),
    key: dto.key,
  };
}
