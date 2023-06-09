import { useCallback } from "react";
import type { CreateUser, User, Credentials } from "@/models";
import {
  type UserRepository,
  fetchUsers,
  postUser,
  fetchCredentials,
  getUser,
  deleteUser,
} from "@/repositories";
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

export function useUser(
  service: UserService,
  loadingDispatcher: LoadingDispatcher,
  userId?: string
) {
  const loadUser = useCallback(() => {
    if (!userId) return Promise.reject("userId is required");
    return getUser(service.userRepository, userId);
  }, [service.userRepository, userId]);

  const [user] = useSimpleLoader<User>(loadingDispatcher, loadUser);

  return {
    user,
  };
}

export function useDeleteUser(service: UserService) {
  const delUser = useCallback(
    (userId?: string) => {
      if (!userId) return;
      return deleteUser(service.userRepository, userId);
    },
    [service.userRepository]
  );

  return {
    deleteUser: delUser,
  };
}

export function useCredentials(
  service: UserService,
  loadingDispatcher: LoadingDispatcher
) {
  const loadCredentials = useCallback(() => {
    return fetchCredentials(service.userRepository);
  }, [service]);

  const [credentials] = useSimpleLoader<Credentials>(
    loadingDispatcher,
    loadCredentials
  );

  return {
    credentials,
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
