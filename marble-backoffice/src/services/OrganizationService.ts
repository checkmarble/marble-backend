import { useCallback, useEffect, useState } from "react";
import type {
  AstNodeEvaluation,
  DataModel,
  Organization,
  Scenario,
  ScenarioValidation,
} from "@/models";
import { PageLink } from "@/models";
import {
  type OrganizationRepository,
  type ScenariosRepository,
  type UserRepository,
  fetchAllOrganizations,
  fetchOrganization,
  postOrganization,
  fetchScenariosOfOrganization,
  fetchScenario,
  deleteOrganization,
  patchOrganization,
  fetchDataModelOfOrganization,
  replaceDataModelOfOrganization,
  postScenario,
  fetchIteration,
  postIteration,
  patchIteration,
  postRule,
  updateRule,
  publishIteration,
} from "@/repositories";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { showLoader, type LoadingDispatcher } from "@/hooks/Loading";
import { useNavigate } from "react-router-dom";
import {
  adaptDataModelApiResult,
  adaptDataModelDto,
} from "@/models/DataModelDto";
import {
  type ExampleRule,
  demoRules,
  exampleTriggerCondition,
} from "./ExampleAst";

export interface OrganizationService {
  organizationRepository: OrganizationRepository;
  scenariosRepository: ScenariosRepository;
  userRepository: UserRepository;
}

export function useAllOrganizations(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher
) {
  const loadAllOrganizations = useCallback(() => {
    return fetchAllOrganizations(service.organizationRepository);
  }, [service]);

  const [allOrganizations, refreshAllOrganizations] = useSimpleLoader<
    Organization[]
  >(loadingDispatcher, loadAllOrganizations);

  return {
    allOrganizations,
    refreshAllOrganizations,
  };
}

export function useOrganization(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadOrganization = useCallback(() => {
    return fetchOrganization(service.organizationRepository, organizationId);
  }, [service, organizationId]);

  const [organization, refreshOrganization] = useSimpleLoader<Organization>(
    loadingDispatcher,
    loadOrganization
  );

  return {
    organization,
    refreshOrganization,
  };
}

export function useCreateOrganization(service: OrganizationService) {
  const createOrganization = useCallback(
    async (name: string) => {
      await postOrganization(service.organizationRepository, {
        name,
        databaseName: name,
      });
    },
    [service]
  );

  return {
    createOrganization,
  };
}

export function useDeleteOrganization(service: OrganizationService) {
  const deleteOrg = useCallback(
    async (organizationId: string) => {
      await deleteOrganization(service.organizationRepository, organizationId);
    },
    [service]
  );

  return { deleteOrganization: deleteOrg };
}

export function useScenarios(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadScenarios = useCallback(() => {
    return fetchScenariosOfOrganization(
      service.scenariosRepository,
      organizationId
    );
  }, [service, organizationId]);

  const [scenarios, refreshScenarios] = useSimpleLoader<Scenario[]>(
    loadingDispatcher,
    loadScenarios
  );

  return {
    scenarios,
    refreshScenarios,
  };
}

export function useSingleScenario(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  scenarioId: string
) {
  const loadScenario = useCallback(async () => {
    const scenario = await fetchScenario(
      service.scenariosRepository,
      scenarioId
    );

    if (scenario.liveVersionId !== null) {
      scenario.liveIteration = await fetchIteration(
        service.scenariosRepository,
        scenario.organizationId,
        scenario.liveVersionId
      );
    }

    // const iterations = fetchIterationsOfScenario(
    //   service.scenariosRepository,
    //   scenario.organizationId,
    //   scenarioId
    // );

    return scenario;
  }, [service, scenarioId]);

  const [scenario, refreshScenario] = useSimpleLoader<Scenario>(
    loadingDispatcher,
    loadScenario
  );

  return {
    scenario,
    refreshScenario,
  };
}

function nodeEvaluationErrors(node: AstNodeEvaluation): string[] {
  return [
    node.evaluationError,
    ...node.children.flatMap(nodeEvaluationErrors),
    ...Object.values(node.namedChildren).flatMap(nodeEvaluationErrors),
  ].filter((v) => v !== "");
}

function scenarioValidationErrors(validation: ScenarioValidation): string[] {
  const errors: string[] = [
    ...validation.errors.map((e) => `Error: ${e}`),
    ...nodeEvaluationErrors(validation.triggerEvaluation).map(
      (e) => `Trigger: ${e}`
    ),
    ...validation.rulesEvaluations
      .flatMap(nodeEvaluationErrors)
      .map((e) => `Rule: ${e}`),
  ];

  return errors;
}

export function useAddScenarios(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string,
  refreshScenarios: () => Promise<void>
) {
  return {
    addDemoScenario: async () => {
      const createDemoScenario = async () => {
        // Post a new scenario
        const scenario = await postScenario(
          service.organizationRepository,
          organizationId,
          {
            name: "Demo scenario",
            description: "Demo scenario",
            triggerObjectType: "transactions",
          }
        );

        // post a new iteration
        const iterationId = (
          await postIteration(
            service.scenariosRepository,
            organizationId,
            scenario.scenarioId
          )
        ).iterationId;

        // patch the iteration
        const { scenarioValidation } = await patchIteration(
          service.scenariosRepository,
          organizationId,
          iterationId,
          {
            scoreReviewThreshold: 20,
            scoreRejectThreshold: 30,
            schedule: "*/10 * * * *",
            triggerCondition: exampleTriggerCondition,
          }
        );

        // check for errors in trigger evaluation
        const errors = nodeEvaluationErrors(
          scenarioValidation.triggerEvaluation
        );
        if (errors.length > 0) {
          throw new Error(errors.join("\n"));
        }

        const createExampleRule = async (exampleRule: ExampleRule) => {
          const rule = await postRule(
            service.scenariosRepository,
            organizationId,
            iterationId
          );

          const { scenarioValidation } = await updateRule(
            service.scenariosRepository,
            organizationId,
            rule.ruleId,
            {
              ...exampleRule,
            }
          );

          const errors = scenarioValidationErrors(scenarioValidation);
          if (errors.length > 0) {
            throw new Error(errors.join("\n"));
          }
        };

        // post a rules
        for (const exampleRule of demoRules) {
          await createExampleRule(exampleRule);
        }

        await publishIteration(
          service.scenariosRepository,
          organizationId,
          iterationId
        );

        await refreshScenarios();
      };

      await showLoader(loadingDispatcher, createDemoScenario());
    },
  };
}

export function useDataModel(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadDataModel = useCallback(async () => {
    const dataModel = await fetchDataModelOfOrganization(
      service.scenariosRepository,
      organizationId
    );
    return dataModel;
  }, [service, organizationId]);

  const [dataModel] = useSimpleLoader<DataModel>(
    loadingDispatcher,
    loadDataModel
  );

  return {
    dataModel,
  };
}

export function useEditDataModel(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string,
  dataModel: DataModel | null
) {
  const [dataModelString, setDataModelString] = useState<string | null>(null);
  const [dataModelError, setDataModelError] = useState<string>("");
  const [validatedDataModel, setValidatedDataModel] =
    useState<DataModel | null>(null);

  const setBackendModel = useCallback((dataModel: DataModel) => {
    setDataModelString(JSON.stringify(adaptDataModelDto(dataModel), null, 2));
  }, []);

  useEffect(() => {
    if (dataModel !== null && dataModelString === null) {
      setBackendModel(dataModel);
    }
  }, [dataModel, dataModelString, setBackendModel]);

  const validateDataModel = useCallback(() => {
    try {
      const data = JSON.parse(dataModelString || "");
      setValidatedDataModel(
        adaptDataModelApiResult({
          data_model: data,
        })
      );
      setDataModelError("");
    } catch (e) {
      setValidatedDataModel(null);
      if (e instanceof Error) {
        setDataModelError(e.message);
      }
    }
  }, [dataModelString]);

  useEffect(() => {
    validateDataModel();
  }, [dataModelString, validateDataModel]);

  const [saveDataModelAlertDialogOpen, setSaveDataModelAlertDialogOpen] =
    useState(false);

  const canSave = validatedDataModel !== null;
  const saveDataModel = async () => {
    if (validatedDataModel === null) {
      return;
    }
    setSaveDataModelAlertDialogOpen(true);
  };

  const saveDataModelConfirmed = async () => {
    if (validatedDataModel === null) {
      throw Error("Cannot save invalid data model");
    }
    const newDataModel = await showLoader(
      loadingDispatcher,
      replaceDataModelOfOrganization(
        service.organizationRepository,
        organizationId,
        validatedDataModel
      )
    );
    setBackendModel(newDataModel);
    setSaveDataModelAlertDialogOpen(false);
  };

  return {
    dataModelString,
    setDataModelString,
    saveDataModel,
    saveDataModelConfirmed,
    dataModelError,
    saveDataModelAlertDialogOpen,
    setSaveDataModelAlertDialogOpen,
    canSave,
  };
}

interface OrganizationViewModel {
  name: string;
  exportScheduledExecutionS3: string;
}

export function useEditOrganization(
  service: OrganizationService,
  loadingDispatcher: LoadingDispatcher,
  initialOrganization: Organization
) {
  const organizationId = initialOrganization.organizationId;

  const navigate = useNavigate();

  const [organizationViewModel, setOrganizationViewModel] =
    useState<OrganizationViewModel>({
      exportScheduledExecutionS3:
        initialOrganization.exportScheduledExecutionS3,
      name: initialOrganization.name,
    });

  const saveOrganization = async () => {
    const newOrganization = await showLoader(
      loadingDispatcher,
      patchOrganization(service.organizationRepository, organizationId, {
        name: organizationViewModel.name,
        exportScheduledExecutionS3:
          organizationViewModel.exportScheduledExecutionS3,
      })
    );
    navigate(PageLink.organizationDetails(newOrganization.organizationId));
  };

  return {
    organizationViewModel,
    setOrganizationViewModel,
    saveOrganization,
  };
}
