import { type MarbleApi } from "@/infra/MarbleApi";
import type { User, CreateUser, Credentials } from "@/models";
import {
  adaptSingleUserApiResult,
  adaptUsersApiResult,
} from "@/models/UserDto";
import { adaptCredential } from "@/models/CredentialsDto";

export interface UserRepository {
  marbleApi: MarbleApi;
}

export async function fetchUsers(
  repository: UserRepository,
  organizationIdFilter?: string
): Promise<User[]> {
  const users = organizationIdFilter
    ? repository.marbleApi.usersOfOrganization(organizationIdFilter)
    : repository.marbleApi.allUsers();
  return adaptUsersApiResult(await users);
}

export async function postUser(
  repositories: UserRepository,
  createUser: CreateUser
): Promise<User> {
  return adaptSingleUserApiResult(
    await repositories.marbleApi.postUser(createUser)
  );
}

export async function getUser(
  repositories: UserRepository,
  userId: string
): Promise<User> {
  return adaptSingleUserApiResult(await repositories.marbleApi.getUser(userId));
}

export async function deleteUser(
  repositories: UserRepository,
  userId: string
): Promise<void> {
  await repositories.marbleApi.deleteUser(userId);
}

export async function fetchCredentials(
  repository: UserRepository
): Promise<Credentials> {
  return adaptCredential(await repository.marbleApi.credentials());
}
