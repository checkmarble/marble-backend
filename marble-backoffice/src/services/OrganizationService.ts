import { useCallback } from "react";
import { type CreateOrganization, type Organization } from "@/models";
import {
  OrganizationRepository,
  fetchAllOrganizations,
  fetchOrganization,
  createOrganization,
} from "@/repositories";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { type LoadingDispatcher } from "@/hooks/Loading";

export class OrganizationService {
  organizationRepository: OrganizationRepository;

  constructor(organizationRepository: OrganizationRepository) {
    this.organizationRepository = organizationRepository;
  }

  async allOrganization() {
    return fetchAllOrganizations(this.organizationRepository);
  }

  async createOrganization(create: CreateOrganization) {
    return createOrganization(this.organizationRepository, create);
  }

  async organizationById(organizationId: string) {
    return fetchOrganization(this.organizationRepository, organizationId);
  }
}

export function useAllOrganizations(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher
) {
  const loadAllOrganizations = useCallback(() => {
    return service.allOrganization();
  }, [service]);

  const [allOrganizations, fetchAllOrganizations] = useSimpleLoader<
    Organization[]
  >(loadingDispatcher, loadAllOrganizations);

  return {
    allOrganizations,
    fetchAllOrganizations,
  };
}

export function useOrganization(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadOrganization = useCallback(() => {
    return service.organizationById(organizationId);
  }, [service, organizationId]);

  const [organization, fetchOrganization] = useSimpleLoader<Organization>(
    loadingDispatcher,
    loadOrganization
  );

  return {
    organization,
    fetchOrganization,
  };
}

export function useCreateOrganization(service: OrganizationService) {
  const createOrganization = useCallback(
    async (name: string) => {
      await service.createOrganization({
        name,
        databaseName: `${name}_database`,
      });
    },
    [service]
  );

  return {
    createOrganization,
  };
}
