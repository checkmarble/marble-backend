import { useCallback, useState } from "react";
import type { Organization } from "@/models";
import { PageLink } from "@/models";
import {
  type OrganizationRepository,
  type UserRepository,
  fetchAllOrganizations,
  fetchOrganization,
  postOrganization,
  deleteOrganization,
  patchOrganization,
} from "@/repositories";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { showLoader, type LoadingDispatcher } from "@/hooks/Loading";
import { useNavigate } from "react-router-dom";

export interface OrganizationService {
  organizationRepository: OrganizationRepository;
  userRepository: UserRepository;
}

export function useAllOrganizations(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher
) {
  const loadAllOrganizations = useCallback(() => {
    return fetchAllOrganizations(service.organizationRepository);
  }, [service]);

  const [allOrganizations, refreshAllOrganizations] = useSimpleLoader<
    Organization[]
  >(loadingDispatcher, loadAllOrganizations);

  return {
    allOrganizations,
    refreshAllOrganizations,
  };
}

export function useOrganization(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadOrganization = useCallback(() => {
    return fetchOrganization(service.organizationRepository, organizationId);
  }, [service, organizationId]);

  const [organization, refreshOrganization] = useSimpleLoader<Organization>(
    loadingDispatcher,
    loadOrganization
  );

  return {
    organization,
    refreshOrganization,
  };
}

export function useCreateOrganization(service: OrganizationService) {
  const createOrganization = useCallback(
    async (name: string) => {
      await postOrganization(service.organizationRepository, {
        name,
        databaseName: name,
      });
    },
    [service]
  );

  return {
    createOrganization,
  };
}

export function useDeleteOrganization(service: OrganizationService) {
  const deleteOrg = useCallback(
    async (organizationId: string) => {
      await deleteOrganization(service.organizationRepository, organizationId);
    },
    [service]
  );

  return { deleteOrganization: deleteOrg };
}

interface OrganizationViewModel {
  name: string;
  exportScheduledExecutionS3: string;
}

export function useEditOrganization(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  initialOrganization: Organization
) {
  const organizationId = initialOrganization.organizationId;

  const navigate = useNavigate();

  const [organizationViewModel, setOrganizationViewModel] =
    useState<OrganizationViewModel>({
      exportScheduledExecutionS3:
        initialOrganization.exportScheduledExecutionS3,
      name: initialOrganization.name,
    });

  const saveOrganization = async () => {
    const newOrganization = await showLoader(
      loadingDispatcher,
      patchOrganization(service.organizationRepository, organizationId, {
        name: organizationViewModel.name,
        exportScheduledExecutionS3:
          organizationViewModel.exportScheduledExecutionS3,
      })
    );
    navigate(PageLink.organizationDetails(newOrganization.organizationId));
  };

  return {
    organizationViewModel,
    setOrganizationViewModel,
    saveOrganization,
  };
}
