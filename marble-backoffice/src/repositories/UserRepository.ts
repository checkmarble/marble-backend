import { MarbleApi } from "@/infra/MarbleApi";
import { adaptCredential, adaptUser } from "@/models";
import type { User, CreateUser, Credentials } from "@/models";
import {
  adaptSingleUserApiResultDto,
  adaptUsersApiResultDto,
} from "@/models/UserDto";
import { adaptCredentialsApiResultDto } from "@/models/CredentialsDto";

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
  const result = adaptUsersApiResultDto(await users);

  return result.users.map(adaptUser);
}

export async function postUser(
  repositories: UserRepository,
  createUser: CreateUser
): Promise<User> {
  const result = adaptSingleUserApiResultDto(
    await repositories.marbleApi.postUser(createUser)
  );
  return adaptUser(result.user);
}

export async function getUser(
  repositories: UserRepository,
  userId: string
): Promise<User> {
  const result = adaptSingleUserApiResultDto(
    await repositories.marbleApi.getUser(userId)
  );
  return adaptUser(result.user);
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
  const dto = adaptCredentialsApiResultDto(
    await repository.marbleApi.credentials()
  );
  return adaptCredential(dto.credentials);
}
