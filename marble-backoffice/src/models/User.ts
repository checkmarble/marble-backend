import { type UserDto } from "./UserDto";

export interface User {
  userId: string;
  email: string;
  role: string;
  organizationId: string;
}

export function adaptUser(dto: UserDto): User {
  return {
    userId: dto.user_id,
    email: dto.email,
    role: dto.role,
    organizationId: dto.organization_id,
  };
}

export interface CreateUser {
  email: string;
  role: string;
  organizationId: string;
}

