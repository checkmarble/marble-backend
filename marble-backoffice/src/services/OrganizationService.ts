import { useCallback, useEffect, useState } from "react";
import { DataModel, Organization, PageLink, Scenario } from "@/models";
import {
  type OrganizationRepository,
  type ScenariosRepository,
  type UserRepository,
  fetchAllOrganizations,
  fetchOrganization,
  postOrganization,
  fetchScenariosOfOrganization,
  deleteOrganization,
  patchOrganization,
  fetchDataModelOfOrganization,
  replaceDataModelOfOrganization,
  postScenario,
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
import { testAst } from "./TestAst";

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
        await patchIteration(
          service.scenariosRepository,
          organizationId,
          iterationId,
          {
            scoreReviewThreshold: 20,
            scoreRejectThreshold: 100,
            // schedule?: string;
            // batchTriggerSql?: string;
          }
        );

        // post a new rule
        const rule = await postRule(
          service.scenariosRepository,
          organizationId,
          iterationId
        );

        const updatedRule = await updateRule(
          service.scenariosRepository,
          organizationId,
          rule.ruleId,
          {
            expression: testAst,
          }
        );

        await publishIteration(
          service.scenariosRepository,
          organizationId,
          iterationId
        );

        console.log(JSON.stringify(updatedRule, null, 2));
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
