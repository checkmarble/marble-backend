import type {
  CreateDecision,
  CreateOrganization,
  PatchOrganization,
  CreateUser,
} from "@/models";
import { HttpMethod } from "./fetchUtils";
import { AuthorizedFetcher } from "./AuthorizedFetcher";
import { AstNodeDto } from "@/models/AstExpressionDto";

const ORGANIZATION_URL_PATH = "organizations";
const SCENARIO_URL_PATH = "scenarios";
const USERS_URL_PATH = "users";
const INGESTION_URL_PATH = "ingestion";
const DECISIONS_URL_PATH = "decisions";
const AST_EXPRESSION_URL_PATH = "ast-expression";
const DATA_MODEL_URL_PATH = "data-model";

export interface IngestObjects {
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

  async sendAuthorizedJson(args: {
    method: HttpMethod;
    path: string;
    body: unknown;
  }): Promise<unknown> {
    const request = new Request(this.apiUrl(args.path), {
      method: args.method,
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

  async postOrganization(
    createOrganizationBody: CreateOrganization
  ): Promise<unknown> {
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: ORGANIZATION_URL_PATH,
      body: {
        name: createOrganizationBody.name,
        databaseName: createOrganizationBody.databaseName,
      },
    });
  }

  async deleteOrganization(organizationId: string): Promise<unknown> {
    const orgIdParam = encodeURIComponent(organizationId);
    return this.deleteAuthorizedJson(`${ORGANIZATION_URL_PATH}/${orgIdParam}`);
  }

  async patchOrganization(
    organizationId: string,
    patchOrganization: PatchOrganization
  ): Promise<unknown> {
    const orgIdParam = encodeURIComponent(organizationId);
    return this.sendAuthorizedJson({
      method: HttpMethod.Patch,
      path: `${ORGANIZATION_URL_PATH}/${orgIdParam}`,
      body: {
        name: patchOrganization.name,
        export_scheduled_execution_s3:
          patchOrganization.exportScheduledExecutionS3,
      },
    });
  }

  async scenariosOfOrganization(organizationId: string): Promise<unknown> {
    return this.getAuthorizedJson(
      urlWithOrganizationId(SCENARIO_URL_PATH, organizationId)
    );
  }

  async dataModelOfOrganization(organizationId: string): Promise<unknown> {
    return this.getAuthorizedJson(
      urlWithOrganizationId(DATA_MODEL_URL_PATH, organizationId)
    );
  }

  async postDataModelOfOrganization(
    organizationId: string,
    dataModel: unknown
  ) {
    return await this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(DATA_MODEL_URL_PATH, organizationId),
      body: {
        data_model: dataModel,
      },
    });
  }

  async scenariosById(scenarioId: string): Promise<unknown> {
    const scenarioIdParam = encodeURIComponent(scenarioId);
    return this.getAuthorizedJson(`${SCENARIO_URL_PATH}/${scenarioIdParam}`);
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
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
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

  async ingest(ingestObject: IngestObjects) {
    const objectTypeParam = encodeURIComponent(ingestObject.tableName);
    await this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: `${INGESTION_URL_PATH}/${objectTypeParam}`,
      body: ingestObject.content,
    });
    return ingestObject;
  }

  async decisions(): Promise<unknown> {
    return await this.getAuthorizedJson(DECISIONS_URL_PATH);
  }

  async postDecision(createDecision: CreateDecision) {
    return await this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: DECISIONS_URL_PATH,
      body: createDecision,
    });
  }

  async astExpressionAvailableFunctions(): Promise<unknown> {
    return await this.getAuthorizedJson(
      `${AST_EXPRESSION_URL_PATH}/available-functions`
    );
  }

  async validateAstExpression(organizationId: string, expression: AstNodeDto) {
    return await this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(
        `${AST_EXPRESSION_URL_PATH}/validate`,
        organizationId
      ),
      body: {
        expression: expression,
      },
    });
  }

  async dryRunAstExpression(organizationId: string, expression: AstNodeDto) {
    return await this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(
        `${AST_EXPRESSION_URL_PATH}/dry-run`,
        organizationId
      ),
      body: {
        expression: expression,
        payload: {
          object_id: "transaction_c",
          account_id: "marble-account-id",
          direction: "payout",
          status: "pending",
          bic_country: "FR",
          amount: 100,
          updated_at: new Date(),
        },
        payload_type: "transactions",
      },
    });
  }

  async editorIdentifiers(scenarioId: string) {
    const scenarioIdParam = encodeURIComponent(scenarioId);
    return await this.getAuthorizedJson(
      `editor/${scenarioIdParam}/identifiers`
    );
  }
}

function urlWithOrganizationId(path: string, organizationId: string): string {
  const r = new URLSearchParams({ "organization-id": organizationId });
  return `${path}?${r.toString()}`;
}
