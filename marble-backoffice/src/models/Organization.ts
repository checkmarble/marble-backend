export interface Organization {
  organizationId: string;
  name: string;
}

export interface CreateOrganization {
  name: string;
  databaseName: string;
}
