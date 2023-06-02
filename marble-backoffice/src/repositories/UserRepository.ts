import { MarbleApi } from "@/infra/MarbleApi";
import { adaptUser } from "@/models";
import type { User, CreateUser } from "@/models";
import {
  adaptSingleUserApiResultDto,
  adaptUsersApiResultDto,
} from "@/models/UserDto";

export interface UserRepository {
  marbleApi: MarbleApi;
}

export async function fetchAllUsers(
  repository: UserRepository
): Promise<User[]> {
  const result = adaptUsersApiResultDto(await repository.marbleApi.allUsers());
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
