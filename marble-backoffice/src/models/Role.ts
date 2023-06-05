export enum Role {
  VIEWER = "VIEWER",
  BUILDER = "BUILDER",
  PUBLISHER = "PUBLISHER",
  ADMIN = "ADMIN",
  API_CLIENT = "API_CLIENT",
  MARBLE_ADMIN = "MARBLE_ADMIN",
}

export function isRole(role: string) : role is Role {
  return role in Role
}

export function adaptRole(s: string) : Role {
  if (isRole(s)) {
    return Role[s]
  }
  throw Error(`Unknown role: ${s}`)
}