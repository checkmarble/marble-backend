import type { MarbleApi } from "@/infra/MarbleApi";
import type {
  Organization,
  CreateOrganization,
  ApiKey,
  PatchOrganization,
} from "@/models";
import {
  adaptOrganizationsApiResult,
  adaptSingleOrganizationApiResult,
} from "@/models/OrganizationDto";
import { adaptApiKeysApiResult } from "@/models/ApiKeyDto";

export interface OrganizationRepository {
  marbleApi: MarbleApi;
}

export async function fetchAllOrganizations(
  repository: OrganizationRepository
): Promise<Organization[]> {
  return adaptOrganizationsApiResult(
    await repository.marbleApi.allOrganizations()
  );
}

export async function postOrganization(
  repository: OrganizationRepository,
  create: CreateOrganization
): Promise<Organization> {
  return adaptSingleOrganizationApiResult(
    await repository.marbleApi.postOrganization({
      name: create.name,
      databaseName: create.databaseName,
    })
  );
}

export async function fetchOrganization(
  repository: OrganizationRepository,
  organizationId: string
): Promise<Organization> {
  return adaptSingleOrganizationApiResult(
    await repository.marbleApi.organizationsById(organizationId)
  );
}

export async function deleteOrganization(
  repository: OrganizationRepository,
  organizationId: string
): Promise<void> {
  await repository.marbleApi.deleteOrganization(organizationId);
}

export async function patchOrganization(
  repository: OrganizationRepository,
  organizationId: string,
  patchOrganization: PatchOrganization
): Promise<Organization> {
  return adaptSingleOrganizationApiResult(
    await repository.marbleApi.patchOrganization(
      organizationId,
      patchOrganization
    )
  );
}

export async function fetchApiKeys(
  repository: OrganizationRepository,
  organizationId: string
): Promise<ApiKey[]> {
  return adaptApiKeysApiResult(
    await repository.marbleApi.apiKeysOfOrganization(organizationId)
  );
}
