import { type Role } from "./Role";


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

