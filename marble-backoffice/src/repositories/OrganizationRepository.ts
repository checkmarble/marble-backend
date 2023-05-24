import { MarbleApi } from "@/infra/MarbleApi";
import { type Organization, adaptOrganization, type CreateOrganization } from "@/models";
import {
  adaptOrganizationsApiResultDto,
  adaptSingleOrganizationApiResultDto,
} from "@/models/OrganizationDto";

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

export async function createOrganization(
  repository: OrganizationRepository,
  create: CreateOrganization,
): Promise<Organization> {
  const created = adaptSingleOrganizationApiResultDto(
    await repository.marbleApi.postOrganization({
      name: create.name,
      databaseName: create.databaseName,
    })
  );

  return adaptOrganization(created.organization);
}
