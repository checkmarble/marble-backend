import { OrganizationDto } from "./OrganizationDto";

export interface Organization {
  organizationId: string;
  name: string;
  dateCreated: Date;
}

export function adaptOrganization(dto: OrganizationDto): Organization {
  return {
    organizationId: dto.organization_id,
    name: dto.name,
    dateCreated: dto.date_created,
  };
}
