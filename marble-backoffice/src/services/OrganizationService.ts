import { Organization } from "@/models";
import { OrganizationRepository, fetchOrganizations } from "@/repositories";
import { useEffect, useState } from "react";

export class OrganizationService {
  organizationRepository: OrganizationRepository;

  constructor(organizationRepository: OrganizationRepository) {
    this.organizationRepository = organizationRepository;
  }

  async allOrganization() {
    return fetchOrganizations(this.organizationRepository);
  }
}

export function useAllOrganizations(service: OrganizationService) {
  const [allOrganizations, setAllOrganizations] = useState<
    Organization[] | null
  >(null);

  useEffect(() => {
    (async () => {
      setAllOrganizations(await service.allOrganization());
    })();
  }, [service]);

  return {
    allOrganizations,
  };
}
