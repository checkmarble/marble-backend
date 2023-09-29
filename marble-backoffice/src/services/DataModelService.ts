import { type LoadingDispatcher, showLoader } from "@/hooks/Loading";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import type { DataModel } from "@/models";
import {
  adaptDataModelDto,
  adaptDataModelApiResult,
} from "@/models/DataModelDto";
import {
  OrganizationRepository,
  fetchDataModelOfOrganization,
  replaceDataModelOfOrganization,
} from "@/repositories";
import { useCallback, useState, useEffect } from "react";

export interface DataModelService {
  organizationRepository: OrganizationRepository;
}

export function useDataModel(
  service: DataModelService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadDataModel = useCallback(async () => {
    const dataModel = await fetchDataModelOfOrganization(
      service.organizationRepository,
      organizationId
    );
    return dataModel;
  }, [service, organizationId]);

  const [dataModel, refreshDataModel] = useSimpleLoader<DataModel>(
    loadingDispatcher,
    loadDataModel
  );

  return {
    dataModel,
    refreshDataModel,
  };
}

export function useEditDataModel(
  service: DataModelService,
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

export function useDeleteDataModel({
  service,
  loadingDispatcher,
  organizationId,
  refreshDataModel,
}: {
  service: DataModelService;
  loadingDispatcher: LoadingDispatcher;
  organizationId: string;
  refreshDataModel: () => Promise<void>;
}) {
  const cleanDataModel = useCallback(async () => {
    await showLoader(
      loadingDispatcher,
      service.organizationRepository.marbleApi.deleteDataModel(organizationId)
    );
    await refreshDataModel();
  }, [
    loadingDispatcher,
    organizationId,
    service.organizationRepository.marbleApi,
    refreshDataModel
  ]);

  return {
    cleanDataModel,
  };
}
