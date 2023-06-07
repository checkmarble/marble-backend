import type { CreateUser } from "@/models";
import { HttpMethod } from "./fetchUtils";
import { AuthorizedFetcher } from "./AuthorizedFetcher";

const ORGANIZATION_URL_PATH = "organizations";
const SCENARIO_URL_PATH = "scenarios";
const USERS_URL_PATH = "users";

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

  async credentials(): Promise<unknown> {
    return this.getAuthorizedJson("credentials");
  }

  async apiKeysOfOrganization(organizationId: string): Promise<unknown> {
    return this.getAuthorizedJson(
      urlWithOrganizationId("apikeys", organizationId)
    );
  }
}

function urlWithOrganizationId(path: string, organizationId: string): string {
  const r = new URLSearchParams({ "organization-id": organizationId });
  return `${path}/?${r.toString()}`;
}
