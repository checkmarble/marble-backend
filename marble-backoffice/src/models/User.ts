import { Role } from "./Role";

export interface User {
  userId: string;
  email: string;
  role: Role;
  organizationId: string;
  firstName: string;
  lastName: string;
}

export interface CreateUser {
  email: string;
  role: Role;
  organizationId: string;
  firstName: string;
  lastName: string;
}
