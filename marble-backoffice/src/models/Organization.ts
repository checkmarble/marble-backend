import { OrganizationDto } from "./OrganizationDto";

export interface Organization {
  organizationId: string;
  name: string;
  dateCreated: Date;
}

export function adaptOrganization(dto: OrganizationDto): Organization {
  return {
    organizationId: dto.id,
    name: dto.name,
    dateCreated: new Date(), // temporary
  };
}


export interface CreateOrganization {
  name: string;
  databaseName: string;
}
