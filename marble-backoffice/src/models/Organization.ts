export interface Organization {
  organizationId: string;
  name: string;
  dateCreated: Date;
}

export interface CreateOrganization {
  name: string;
  databaseName: string;
}
