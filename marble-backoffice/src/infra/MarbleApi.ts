import type {
  CreateDecision,
  CreateOrganization,
  PatchOrganization,
  CreateUser,
  CreateScenario,
  UpdateRule,
  UpdateIteration,
  AstNode,
  CreateDataModelTable,
  CreateDataModelField,
  CreateDataModelLink,
} from "@/models";
import { HttpMethod } from "./fetchUtils";
import type { AuthorizedFetcher } from "./AuthorizedFetcher";
import { adaptAstNodeDto } from "@/models/AstExpressionDto";

const ORGANIZATION_URL_PATH = "organizations";
const SCENARIO_URL_PATH = "scenarios";
const USERS_URL_PATH = "users";
const INGESTION_URL_PATH = "ingestion";
const DECISIONS_URL_PATH = "decisions";
const AST_EXPRESSION_URL_PATH = "ast-expression";
const DATA_MODEL_URL_PATH = "data-model";
const SCENARIO_ITERATIONS_URL_PATH = "scenario-iterations";
const SCENARIO_ITERATIONS_RULES_URL_PATH = "scenario-iteration-rules";
const SCENARIO_PUBLICATIONS_URL_PATH = "scenario-publications";
const SCHEDULED_EXECUTIONS_URL_PATH = "scheduled-executions";

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

    return this.fetcher.authorizedJson(request);
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

    return this.fetcher.authorizedJson(request);
  }

  async deleteAuthorizedJson(path: string): Promise<unknown> {
    const request = new Request(this.apiUrl(path), {
      method: HttpMethod.Delete,
    });

    return this.fetcher.authorizedJson(request);
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

  async postDataModelTable(
    organizationId: string,
    createDataModelTable: CreateDataModelTable
  ): Promise<unknown> {
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(
        `${DATA_MODEL_URL_PATH}/tables`,
        organizationId
      ),
      body: {
        name: createDataModelTable.tableName,
        description: createDataModelTable.description,
      },
    });
  }

  async postDataModelField(
    organizationId: string,
    createDataModelField: CreateDataModelField
  ): Promise<unknown> {
    const tableIdParam = encodeURIComponent(createDataModelField.tableId);
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(
        `${DATA_MODEL_URL_PATH}/tables/${tableIdParam}/fields`,
        organizationId
      ),
      body: {
        name: createDataModelField.fieldName,
        description: createDataModelField.description,
        type: createDataModelField.dataType,
        nullable: createDataModelField.nullable,
      },
    });
  }

  async postDataModelLink(
    organizationId: string,
    createDataModelLink: CreateDataModelLink
  ): Promise<unknown> {
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(
        `${DATA_MODEL_URL_PATH}/links`,
        organizationId
      ),
      body: {
        name: createDataModelLink.linkName,
        parent_table_id: createDataModelLink.parentTableId,
        parent_field_id: createDataModelLink.parentFieldId,
        child_table_id: createDataModelLink.childTableId,
        child_field_id: createDataModelLink.childFieldID,
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
      urlWithOrganizationId(`${DATA_MODEL_URL_PATH}`, organizationId)
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

  async editorIdentifiers(scenarioId: string) {
    const scenarioIdParam = encodeURIComponent(scenarioId);
    return await this.getAuthorizedJson(
      `editor/${scenarioIdParam}/identifiers`
    );
  }

  async postScenario(organizationId: string, createScenario: CreateScenario) {
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(SCENARIO_URL_PATH, organizationId),
      body: {
        name: createScenario.name,
        description: createScenario.description,
        triggerObjectType: createScenario.triggerObjectType,
      },
    });
  }

  async fetchIterationById(organizationId: string, iterationId: string) {
    return this.getAuthorizedJson(
      urlWithOrganizationId(
        `${SCENARIO_ITERATIONS_URL_PATH}/${iterationId}`,
        organizationId
      )
    );
  }

  async listIterations(organizationId: string, scenarioId: string) {
    const r = new URLSearchParams({
      scenarioId: scenarioId,
      "organization-id": organizationId,
    });
    const path = `${SCENARIO_ITERATIONS_URL_PATH}?${r.toString()}`;
    return this.getAuthorizedJson(path);
  }

  async postIteration(organizationId: string, scenarioId: string) {
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(
        `${SCENARIO_ITERATIONS_URL_PATH}`,
        organizationId
      ),
      body: {
        scenarioId: scenarioId,
        body: {},
      },
    });
  }

  async patchIteration(
    organizationId: string,
    iterationId: string,
    changes: UpdateIteration
  ) {
    return this.sendAuthorizedJson({
      method: HttpMethod.Patch,
      path: urlWithOrganizationId(
        `${SCENARIO_ITERATIONS_URL_PATH}/${iterationId}`,
        organizationId
      ),
      body: {
        body: {
          trigger_condition_ast_expression:
            changes.triggerCondition === undefined
              ? undefined
              : adaptAstNodeDto(changes.triggerCondition),
          scoreReviewThreshold: changes.scoreReviewThreshold,
          scoreRejectThreshold: changes.scoreRejectThreshold,
          schedule: changes.schedule,
          batchTriggerSQL: changes?.batchTriggerSql,
        },
      },
    });
  }

  async postRule(organizationId: string, iterationId: string) {
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(
        `${SCENARIO_ITERATIONS_RULES_URL_PATH}`,
        organizationId
      ),
      body: {
        scenarioIterationId: iterationId,
      },
    });
  }

  async patchRule(organizationId: string, ruleId: string, changes: UpdateRule) {
    return this.sendAuthorizedJson({
      method: HttpMethod.Patch,
      path: urlWithOrganizationId(
        `${SCENARIO_ITERATIONS_RULES_URL_PATH}/${ruleId}`,
        organizationId
      ),
      body: {
        name: changes?.name,
        description: changes?.description,
        formula_ast_expression:
          changes.formula === undefined
            ? undefined
            : adaptAstNodeDto(changes.formula),
        displayOrder: changes?.displayOrder,
        scoreModifier: changes?.scoreModifier,
      },
    });
  }

  validateIterationPath(iterationId: string) {
    const iterationsIdParam = encodeURIComponent(iterationId);
    return `${SCENARIO_ITERATIONS_URL_PATH}/${iterationsIdParam}/validate`;
  }

  async validateIteration(iterationId: string) {
    return this.getAuthorizedJson(this.validateIterationPath(iterationId));
  }

  async validateScenarioIterationWithGivenTriggerOrRule(
    iterationId: string,
    triggerOrRule: AstNode,
    ruleId: string | null
  ) {
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: this.validateIterationPath(iterationId),
      body: {
        trigger_or_rule: adaptAstNodeDto(triggerOrRule),
        rule_id: ruleId,
      },
    });
  }

  async postScenarioIterationPublication(
    organizationId: string,
    iterationId: string,
    publish: "publish" | "unpublish"
  ) {
    return this.sendAuthorizedJson({
      method: HttpMethod.Post,
      path: urlWithOrganizationId(
        `${SCENARIO_PUBLICATIONS_URL_PATH}`,
        organizationId
      ),
      body: {
        scenarioIterationID: iterationId,
        publicationAction: publish,
      },
    });
  }

  async deleteDataModel(organizationId: string) {
    const url = urlWithOrganizationId(DATA_MODEL_URL_PATH, organizationId);
    const request = new Request(this.apiUrl(url), {
      method: HttpMethod.Delete,
    });

    return this.fetcher.authorizedJson(request);
  }

  async scheduleExecutionOfOrganization({
    organizationId,
  }: {
    organizationId: string;
  }) {
    return this.getAuthorizedJson(
      urlWithOrganizationId(SCHEDULED_EXECUTIONS_URL_PATH, organizationId)
    );
  }

  decisionsOfScheduleExecution({
    organizationId,
    scheduleExecutionId,
  }: {
    organizationId: string;
    scheduleExecutionId: string;
  }): Promise<Blob> {
    const scheduleExecutionIdParam = encodeURIComponent(scheduleExecutionId);
    const path = urlWithOrganizationId(
      `${SCHEDULED_EXECUTIONS_URL_PATH}/${scheduleExecutionIdParam}/decisions.zip`,
      organizationId
    );

    const request = new Request(this.apiUrl(path), {
      method: HttpMethod.Get,
    });

    return this.fetcher.authorizedBlob(request);
  }
}

function urlWithOrganizationId(path: string, organizationId: string): string {
  const r = new URLSearchParams({ "organization-id": organizationId });
  return `${path}?${r.toString()}`;
}
