export interface Organization {
  organizationId: string;
  name: string;
  databaseName: string;
  exportScheduledExecutionS3: string;
}

export interface CreateOrganization {
  name: string;
  databaseName: string;
}

export interface PatchOrganization {
  name?: string;
  exportScheduledExecutionS3?: string;
}
