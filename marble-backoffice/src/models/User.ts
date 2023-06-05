import { type UserDto } from "./UserDto";
import { Role, adaptRole } from "./Role";

export interface User {
  userId: string;
  email: string;
  role: Role;
  organizationId: string;
}

export function adaptUser(dto: UserDto): User {
  return {
    userId: dto.user_id,
    email: dto.email,
    role: adaptRole(dto.role),
    organizationId: dto.organization_id,
  };
}

export interface CreateUser {
  email: string;
  role: Role;
  organizationId: string;
}

