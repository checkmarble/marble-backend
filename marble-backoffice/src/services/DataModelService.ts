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
    isEnum,
  }: {
    parent: string;
    name: string;
    dataType?: DataModelType;
    description?: string;
    nullable?: boolean;
    isEnum?: boolean;
  }) =>
    createDataModelField(repository, organizationId, {
      tableId: parent,
      fieldName: name,
      dataType: dataType ?? "String",
      description: description ?? "",
      nullable: nullable ?? true,
      isEnum: isEnum ?? false,
    });

  // transactions
  const transactionTableId = await createDataModelTable(
    repository,
    organizationId,
    {
      tableName: "transactions",
      description: "All transactions linked to an account or a company",
    }
  );

  const fieldTransactionAccountId = await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "account_id",
    description: "Account that sent or received the transaction",
  });
  await createField({
    parent: transactionTableId,
    dataType: "Bool",
    name: "card_is_3DS",
    description:
      "Flag indicates if the card transaction has been secured by 3DS",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "card_merchant_id",
    description: "Merchant code of a transaction",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "card_merchant_name",
    description: "Merchant name provided by the card scheme",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "card_merchant_country",
    description: "Country of the merchant, for card transaction only",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "card_payment_currency",
    description: "ISO code of the currency used for a card transaction",
    isEnum: true,
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "card_payment_type",
    description:
      "Indicates how the card transaction was processed and valided. Allow to separate online and physical transactions, as well as reccuring ones",
    isEnum: true,
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "category",
    description:
      "MCC code of a card transaction. Empty for transfers and direct debit",
    isEnum: true,
  });
  const fieldTransactionCompanyId = await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "company_id",
    description: "Company linked to the transaction (sender or receiver)",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "counterparty_iban",
    description:
      "IBAN of a counterparty (sender of beneficiary), used for transfers only",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "counterparty_bic",
    description:
      "BIC of a counterparty (sender of beneficiary), used for transfers only",
  });
  await createField({
    parent: transactionTableId,
    dataType: "Timestamp",
    name: "created_at",
    description: "Date of creation of the transaction in the system",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "creditor_identifier",
    description:
      "Information provided by the creditor when sending a direct debit to the account",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "direction",
    description: "Direction of the transaction. Can be PAYIN or PAYOUT",
    isEnum: true,
  });
  await createField({
    parent: transactionTableId,
    dataType: "Bool",
    name: "is_recurring",
    description:
      "Flag used for direct debit mandates, indicates if a direct debit is indicated as being recuring",
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "payment_method",
    description:
      "Type of transaction. Can be : TRANSFER, CARD, DIRECT_DEBIT, WALLET_TRANSFER",
    isEnum: true,
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "scheme",
    description:
      "Indicates the type of direct debit received. Can be B2B or CORE",
    isEnum: true,
  });
  await createField({
    parent: transactionTableId,
    dataType: "String",
    name: "status",
    description:
      "Status of the transaction. Can be : PENDING, VALIDATED, CANCELED",
    isEnum: true,
  });
  await createField({
    parent: transactionTableId,
    dataType: "Timestamp",
    name: "transaction_at",
    description: "Timestamp of the transaction",
  });
  await createField({
    parent: transactionTableId,
    dataType: "Float",
    name: "value",
    description: "Amount of the transaction in EUR",
  });

  // accounts
  const accountTableId = await createDataModelTable(
    repository,
    organizationId,
    {
      tableName: "accounts",
      description: "Bank accounts owned by the company",
    }
  );

  const fieldAccountCompanyId = await createField({
    parent: accountTableId,
    name: "company_id",
    description: "Company owning the account",
  });
  await createField({
    parent: accountTableId,
    name: "BIC",
    dataType: "String",
    description: "BIC of the company's account",
  });
  await createField({
    parent: accountTableId,
    name: "IBAN",
    dataType: "String",
    description: "IBAN of the company's account",
  });
  await createField({
    parent: accountTableId,
    name: "past_balalnce",
    dataType: "Float",
    description: "Balance before the last update",
  });
  await createField({
    parent: accountTableId,
    name: "balance",
    dataType: "Float",
    description: "Account balance in EUR",
  });
  await createField({
    parent: accountTableId,
    name: "created_at",
    dataType: "Timestamp",
    description: "Account date of creation",
  });

  // companies
  const compagnyTableId = await createDataModelTable(
    repository,
    organizationId,
    {
      tableName: "companies",
      description: "Contains the info on the company details",
    }
  );
  await createField({
    parent: compagnyTableId,
    dataType: "String",
    name: "AML_score",
    description: "Sensitivy score to money laundering risks",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "String",
    name: "zip",
    description: "Zip code used for the company main office",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "String",
    name: "activity_type",
    description: "NAF code. Main activity declared for the company",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "String",
    name: "city",
    description: "City where the company main office is registered",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "String",
    name: "country",
    description: "Country of incorporation",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "Timestamp",
    name: "created_at",
    description: "Date of creation in the system",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "String",
    name: "legal_form",
    description: "Legal form registered for the company",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "String",
    name: "name",
    description: "Company legal name",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "Timestamp",
    name: "registration_month",
    description: "Date of registration of the company",
  });
  await createField({
    parent: compagnyTableId,
    dataType: "String",
    name: "registration_number",
    description: "Identification in the local commercial register",
  });

  // users
  const userTableId = await createDataModelTable(repository, organizationId, {
    tableName: "users",
    description: "Information on the users attached to each Company",
  });

  await createField({
    parent: userTableId,
    dataType: "String",
    name: "city",
    description: "City of residence",
  });
  const fieldUserCompanyId = await createField({
    parent: userTableId,
    dataType: "String",
    name: "company_id",
    description: "Link to parent company",
  });
  await createField({
    parent: userTableId,
    dataType: "Timestamp",
    name: "created_at",
    description: "Date of creation in the system",
  });
  await createField({
    parent: userTableId,
    dataType: "String",
    name: "device_token",
    description: "Device otken associated with the user",
  });
  await createField({
    parent: userTableId,
    dataType: "String",
    name: "email",
    description: "Email address of the user",
  });
  await createField({
    parent: userTableId,
    dataType: "Bool",
    name: "is_frozen",
    description: "Flag for users frozen by the system",
  });
  await createField({
    parent: userTableId,
    dataType: "Bool",
    name: "is_pep",
    description: "Flat for users identified as PEP",
  });
  await createField({
    parent: userTableId,
    dataType: "Bool",
    name: "is_under_sanctions",
    description: "Flag for users under sanctions",
  });
  await createField({
    parent: userTableId,
    dataType: "String",
    name: "name",
    description: "Name of the user",
  });
  await createField({
    parent: userTableId,
    dataType: "String",
    name: "phone",
    description: "Phone number of the user",
  });
  await createField({
    parent: userTableId,
    dataType: "String",
    name: "zip",
    description: "Zip of residence of the user",
  });
  await createField({
    parent: userTableId,
    dataType: "String",
    name: "country",
    description: "ISO code of the country of residence of the user",
  });

  // links
  const datamodel = await fetchDataModelOfOrganization(
    repository,
    organizationId
  );
  const fieldAccountObjectId =
    datamodel.tables["accounts"].fields["object_id"].fieldId;
  const fieldCompaniesObjectId =
    datamodel.tables["companies"].fields["object_id"].fieldId;

  // links on transactions
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
    childTableId: transactionTableId,
    childFieldID: fieldTransactionCompanyId,
  });

  // links on accounts
  await createDataModelLink(repository, organizationId, {
    linkName: "company",
    parentTableId: compagnyTableId,
    parentFieldId: fieldCompaniesObjectId,
    childTableId: accountTableId,
    childFieldID: fieldAccountCompanyId,
  });

  // links on users
  await createDataModelLink(repository, organizationId, {
    linkName: "company",
    parentTableId: compagnyTableId,
    parentFieldId: fieldCompaniesObjectId,
    childTableId: userTableId,
    childFieldID: fieldUserCompanyId,
  });
}
