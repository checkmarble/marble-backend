import { type LoadingDispatcher, showLoader } from "@/hooks/Loading";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import type { DataModel, DataModelType } from "@/models";
import {
  DataModelRepository,
  createDataModelField,
  createDataModelTable,
  createDataModelLink,
  fetchDataModelOfOrganization,
} from "@/repositories";
import { useCallback, useState } from "react";

export interface DataModelService {
  dataModelRepository: DataModelRepository;
}

export function useDataModel(
  service: DataModelService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadDataModel = useCallback(async () => {
    const dataModel = await fetchDataModelOfOrganization(
      service.dataModelRepository,
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

// export function useEditDataModel(
//   service: DataModelService,
//   loadingDispatcher: LoadingDispatcher,
//   organizationId: string,
//   dataModel: DataModel | null
// ) {
//   const [dataModelString, setDataModelString] = useState<string | null>(null);
//   const [dataModelError, setDataModelError] = useState<string>("");
//   const [validatedDataModel, setValidatedDataModel] =
//     useState<DataModel | null>(null);

//   const setBackendModel = useCallback((dataModel: DataModel) => {
//     setDataModelString(JSON.stringify(adaptDataModelDto(dataModel), null, 2));
//   }, []);

//   useEffect(() => {
//     if (dataModel !== null && dataModelString === null) {
//       setBackendModel(dataModel);
//     }
//   }, [dataModel, dataModelString, setBackendModel]);

//   const validateDataModel = useCallback(() => {
//     try {
//       const data = JSON.parse(dataModelString || "");
//       setValidatedDataModel(
//         adaptDataModelApiResult({
//           data_model: data,
//         })
//       );
//       setDataModelError("");
//     } catch (e) {
//       setValidatedDataModel(null);
//       if (e instanceof Error) {
//         setDataModelError(e.message);
//       }
//     }
//   }, [dataModelString]);

//   useEffect(() => {
//     validateDataModel();
//   }, [dataModelString, validateDataModel]);

//   const [saveDataModelAlertDialogOpen, setSaveDataModelAlertDialogOpen] =
//     useState(false);

//   const canSave = validatedDataModel !== null;
//   const saveDataModel = async () => {
//     if (validatedDataModel === null) {
//       return;
//     }
//     setSaveDataModelAlertDialogOpen(true);
//   };

//   const saveDataModelConfirmed = async () => {
//     if (validatedDataModel === null) {
//       throw Error("Cannot save invalid data model");
//     }
//     const newDataModel = await showLoader(
//       loadingDispatcher,
//       replaceDataModelOfOrganization(
//         service.dataModelRepository,
//         organizationId,
//         validatedDataModel
//       )
//     );
//     setBackendModel(newDataModel);
//     setSaveDataModelAlertDialogOpen(false);
//   };

//   return {
//     dataModelString,
//     setDataModelString,
//     saveDataModel,
//     saveDataModelConfirmed,
//     dataModelError,
//     saveDataModelAlertDialogOpen,
//     setSaveDataModelAlertDialogOpen,
//     canSave,
//   };
// }

export function useEditDataModel({
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
  const [cleanDataModelAlertDialogOpen, setCleanDataModelAlertDialogOpen] =
    useState(false);

  const repository = service.dataModelRepository;

  const cleanDataModelConfirmed = useCallback(async () => {
    await showLoader(
      loadingDispatcher,
      repository.marbleApi.deleteDataModel(organizationId)
    );
    await refreshDataModel();
    setCleanDataModelAlertDialogOpen(false);
  }, [
    loadingDispatcher,
    organizationId,
    repository.marbleApi,
    refreshDataModel,
  ]);

  const createDemoDataModel = useCallback(async () => {
    await showLoader(
      loadingDispatcher,
      _createDemoDataModel({
        repository,
        organizationId,
      })
    );
    await refreshDataModel();
  }, [loadingDispatcher, organizationId, refreshDataModel, repository]);

  return {
    cleanDataModelConfirmed,
    cleanDataModelAlertDialogOpen,
    setCleanDataModelAlertDialogOpen,
    createDemoDataModel,
  };
}

async function _createDemoDataModel({
  repository,
  organizationId,
}: {
  repository: DataModelRepository;
  organizationId: string;
}) {
  const createField = ({
    parent,
    name,
    dataType,
    description,
    nullable,
  }: {
    parent: string;
    name: string;
    dataType?: DataModelType;
    description?: string;
    nullable?: boolean;
  }) =>
    createDataModelField(repository, organizationId, {
      tableId: parent,
      fieldName: name,
      dataType: dataType ?? "String",
      description: description ?? "",
      nullable: nullable ?? true,
    });

  // transactions
  const transactionTableId = await createDataModelTable(
    repository,
    organizationId,
    {
      tableName: "transactions",
      description: "Transactions of all companies",
    }
  );

  const fieldTransactionAccountId = await createField({
    parent: transactionTableId,
    name: "account_id",
    description: "Account that received the transaction",
  });
  await createField({ parent: transactionTableId, name: "bic_country" });
  await createField({ parent: transactionTableId, name: "country" });
  await createField({ parent: transactionTableId, name: "description" });
  await createField({ parent: transactionTableId, name: "direction" });
  await createField({ parent: transactionTableId, name: "status" });
  await createField({ parent: transactionTableId, name: "title" });
  await createField({
    parent: transactionTableId,
    name: "amount",
    dataType: "Int",
  });

  // accounts

  // "balance":    {DataType: models.Float, Nullable: true},
  // "company_id": {DataType: models.String, Nullable: true},
  // "name":       {DataType: models.String, Nullable: true},
  // "currency":   {DataType: models.String, Nullable: true},
  // "is_frozen":  {DataType: models.Bool},

  const accountTableId = await createDataModelTable(
    repository,
    organizationId,
    {
      tableName: "accounts",
      description: "accounts of the clients",
    }
  );

  await createField({
    parent: accountTableId,
    name: "balance",
    dataType: "Int",
  });

  const fieldAccountCompanyId = await createField({
    parent: accountTableId,
    name: "company_id",
  });
  await createField({ parent: accountTableId, name: "name" });
  await createField({ parent: accountTableId, name: "currency" });
  await createField({
    parent: accountTableId,
    name: "is_frozen",
    nullable: false,
  });

  // companies
  const compagnyTableId = await createDataModelTable(
    repository,
    organizationId,
    {
      tableName: "companies",
      description: "",
    }
  );
  await createField({ parent: compagnyTableId, name: "name" });

  // links

  const datamodel = await fetchDataModelOfOrganization(
    repository,
    organizationId
  );
  const fieldAccountObjectId =
    datamodel.tables["accounts"].fields["object_id"].fieldId;
  const fieldCompaniesObjectId =
    datamodel.tables["companies"].fields["object_id"].fieldId;

  await createDataModelLink(repository, organizationId, {
    linkName: "account",
    parentTableId: accountTableId,
    parentFieldId: fieldAccountObjectId,
    childTableId: transactionTableId,
    childFieldID: fieldTransactionAccountId,
  });

  await createDataModelLink(repository, organizationId, {
    linkName: "company",
    parentTableId: compagnyTableId,
    parentFieldId: fieldCompaniesObjectId,
    childTableId: accountTableId,
    childFieldID: fieldAccountCompanyId,
  });
}
