import { type Role } from "./Role";

export interface ApiKey {
  organizationId: string;
  role: Role;
  key: string;
}
