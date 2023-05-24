import { useCallback, useEffect, useState } from "react";
import { type CreateOrganization, type Organization } from "@/models";
import {
  OrganizationRepository,
  fetchOrganizations,
  createOrganization,
} from "@/repositories";

export class OrganizationService {
  organizationRepository: OrganizationRepository;

  constructor(organizationRepository: OrganizationRepository) {
    this.organizationRepository = organizationRepository;
  }

  async allOrganization() {
    return fetchOrganizations(this.organizationRepository);
  }

  async createOrganization(create: CreateOrganization) {
    return createOrganization(this.organizationRepository, create);
  }
}

export function useAllOrganizations(service: OrganizationService) {
  const [allOrganizations, setAllOrganizations] = useState<
    Organization[] | null
  >(null);

  const fetchAllOrganizations = useCallback(async () => {
    setAllOrganizations(await service.allOrganization());
  }, [service]);
  
  useEffect(() => {
    fetchAllOrganizations()
  }, [fetchAllOrganizations]);


  return {
    allOrganizations,
    fetchAllOrganizations,
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
