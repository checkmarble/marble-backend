import { useCallback } from "react";
import type { CreateUser, User } from "@/models";
import { type UserRepository, fetchAllUsers, postUser } from "@/repositories";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { type LoadingDispatcher } from "@/hooks/Loading";

export interface UserService {
  userRepository: UserRepository;
}

export function useAllUsers(
  service: UserService,
  loadingDispatcher: LoadingDispatcher
) {
  const loadUsers = useCallback(() => {
    return fetchAllUsers(service.userRepository);
  }, [service]);

  const [users, refreshUsers] = useSimpleLoader<User[]>(
    loadingDispatcher,
    loadUsers
  );

  return {
    users,
    refreshUsers,
  };
}

export function useCreateUser(service: UserService) {
  const createUser = useCallback(
    async (createUser: CreateUser) => {
      await postUser(service.userRepository, createUser);
    },
    [service]
  );

  return {
    createUser,
  };
}
