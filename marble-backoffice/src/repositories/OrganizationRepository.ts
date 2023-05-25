import { MarbleApi } from "@/infra/MarbleApi";
import {
  type Organization,
  adaptOrganization,
  type CreateOrganization,
} from "@/models";
import {
  adaptOrganizationsApiResultDto,
  adaptSingleOrganizationApiResultDto,
} from "@/models/OrganizationDto";

export interface OrganizationRepository {
  marbleApi: MarbleApi;
}

export async function fetchAllOrganizations(
  repository: OrganizationRepository
): Promise<Organization[]> {
  const dtos = adaptOrganizationsApiResultDto(
    await repository.marbleApi.allOrganizations()
  );
  return dtos.organizations.map(adaptOrganization);
}

export async function createOrganization(
  repository: OrganizationRepository,
  create: CreateOrganization
): Promise<Organization> {
  const created = adaptSingleOrganizationApiResultDto(
    await repository.marbleApi.postOrganization({
      name: create.name,
      databaseName: create.databaseName,
    })
  );

  return adaptOrganization(created.organization);
}

export async function fetchOrganization(
  repository: OrganizationRepository,
  organizationId: string
): Promise<Organization> {
  const result = adaptSingleOrganizationApiResultDto(
    await repository.marbleApi.organizationsById(organizationId)
  );
  return adaptOrganization(result.organization);
}
