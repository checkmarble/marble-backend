import { useCallback } from "react";
import type { CreateUser, User } from "@/models";
import { type UserRepository, fetchUsers, postUser } from "@/repositories";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { type LoadingDispatcher } from "@/hooks/Loading";

export interface UserService {
  userRepository: UserRepository;
}

export function useUsers(
  service: UserService,
  loadingDispatcher: LoadingDispatcher,
  organizationIdFilter?: string
) {
  const loadUsers = useCallback(() => {
    return fetchUsers(service.userRepository, organizationIdFilter);
  }, [service, organizationIdFilter]);

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
