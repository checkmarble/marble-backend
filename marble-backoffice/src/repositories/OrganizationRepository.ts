import { MarbleApi } from "@/infra/MarbleApi";
import { Organization, adaptOrganization } from "@/models";
import { adaptOrganizationsApiResultDto } from "@/models/OrganizationDto";

export interface OrganizationRepository {
  marbleApi: MarbleApi;
}

export async function fetchOrganizations(
  repository: OrganizationRepository
): Promise<Organization[]> {
  const dtos = adaptOrganizationsApiResultDto(
    await repository.marbleApi.organizations()
  );
  return dtos.organizations.map(adaptOrganization);
}
