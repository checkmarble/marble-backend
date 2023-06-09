import type { CreateUser } from "@/models";
import { HttpMethod } from "./fetchUtils";
import { AuthorizedFetcher } from "./AuthorizedFetcher";

const ORGANIZATION_URL_PATH = "organizations";
const SCENARIO_URL_PATH = "scenarios";
const USERS_URL_PATH = "users";
const INGESTION_URL_PATH = "ingestion";

export interface IngestObject {
  tableName: string;
  content: Record<string, unknown>;
}

export class MarbleApi {
  baseUrl: URL;
  fetcher: AuthorizedFetcher;

  constructor(baseUrl: URL, fetcher: AuthorizedFetcher) {
    this.fetcher = fetcher;
    this.baseUrl = baseUrl;
  }

  apiUrl(path: string): URL {
    return new URL(path, this.baseUrl);
  }

  async getAuthorizedJson(path: string): Promise<unknown> {
    const request = new Request(this.apiUrl(path), {
      method: HttpMethod.Get,
    });

    return this.fetcher.authorizedApiFetch(request);
  }

  async postAuthorizedJson(args: {
    path: string;
    body: unknown;
  }): Promise<unknown> {
    const request = new Request(this.apiUrl(args.path), {
      method: HttpMethod.Post,
      body: JSON.stringify(args.body),
      headers: {
        "Content-type": "application/json; charset=UTF-8",
      },
    });

    return this.fetcher.authorizedApiFetch(request);
  }

  async deleteAuthorizedJson(path: string): Promise<unknown> {
    const request = new Request(this.apiUrl(path), {
      method: HttpMethod.Delete,
    });

    return this.fetcher.authorizedApiFetch(request);
  }

  async allOrganizations(): Promise<unknown> {
    return this.getAuthorizedJson(ORGANIZATION_URL_PATH);
  }

  async organizationsById(organizationId: string): Promise<unknown> {
    const orgIdParam = encodeURIComponent(organizationId);
    return this.getAuthorizedJson(`${ORGANIZATION_URL_PATH}/${orgIdParam}`);
  }

  async postOrganization(createOrganizationBody: unknown): Promise<unknown> {
    return this.postAuthorizedJson({
      path: ORGANIZATION_URL_PATH,
      body: createOrganizationBody,
    });
  }

  async deleteOrganization(organizationId: string): Promise<unknown> {
    const orgIdParam = encodeURIComponent(organizationId);
    return this.deleteAuthorizedJson(`${ORGANIZATION_URL_PATH}/${orgIdParam}`);
  }

  async scenariosOfOrganization(organizationId: string): Promise<unknown> {
    return this.getAuthorizedJson(
      urlWithOrganizationId(SCENARIO_URL_PATH, organizationId)
    );
  }

  async allUsers(): Promise<unknown> {
    return this.getAuthorizedJson(USERS_URL_PATH);
  }

  async usersOfOrganization(organizationId: string): Promise<unknown> {
    const orgIdParam = encodeURIComponent(organizationId);
    return this.getAuthorizedJson(
      `${ORGANIZATION_URL_PATH}/${orgIdParam}/users`
    );
  }

  async postUser(createUser: CreateUser): Promise<unknown> {
    return this.postAuthorizedJson({
      path: USERS_URL_PATH,
      body: {
        email: createUser.email,
        role: createUser.role,
        organization_id: createUser.organizationId,
      },
    });
  }

  async getUser(userId: string): Promise<unknown> {
    const userIdParam = encodeURIComponent(userId);
    return this.getAuthorizedJson(`${USERS_URL_PATH}/${userIdParam}`);
  }

  async deleteUser(userId: string): Promise<unknown> {
    const userIdParam = encodeURIComponent(userId);
    return this.deleteAuthorizedJson(`${USERS_URL_PATH}/${userIdParam}`);
  }

  async credentials(): Promise<unknown> {
    return this.getAuthorizedJson("credentials");
  }

  async apiKeysOfOrganization(organizationId: string): Promise<unknown> {
    return this.getAuthorizedJson(
      urlWithOrganizationId("apikeys", organizationId)
    );
  }

  async ingest(ingestObject: IngestObject) {
    const objectTypeParam = encodeURIComponent(ingestObject.tableName);
    await this.postAuthorizedJson({
      path: `${INGESTION_URL_PATH}/${objectTypeParam}`,
      body: ingestObject.content,
    });
    return ingestObject;
  }
}

function urlWithOrganizationId(path: string, organizationId: string): string {
  const r = new URLSearchParams({ "organization-id": organizationId });
  return `${path}/?${r.toString()}`;
}
