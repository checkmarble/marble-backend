import { MarbleApi } from "@/infra/MarbleApi";
import type { Organization, CreateOrganization, ApiKey } from "@/models";
import {
  adaptOrganizationsApiResultDto,
  adaptSingleOrganizationApiResultDto,
} from "@/models/OrganizationDto";
import { adaptApiKeysResultDto } from "@/models/ApiKeyDto";
import { adaptOrganization, adaptApiKey } from "@/models";

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

export async function postOrganization(
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

export async function fetchApiKeys(
  repository: OrganizationRepository,
  organizationId: string,
): Promise<ApiKey[]> {
  const dto = adaptApiKeysResultDto(
    await repository.marbleApi.apiKeysOfOrganization(organizationId)
  );
  return dto.api_keys.map(adaptApiKey);
}
