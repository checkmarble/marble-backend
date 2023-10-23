export interface LinkToSingle {
  linkedTableName: string;
  parentFieldName: string;
  childFieldName: string;
}

export type DataModelType =
  | "Bool"
  | "Int"
  | "Float"
  | "String"
  | "Timestamp"
  | "unknown";

export interface DataModelField {
  fieldId: string;
  dataType: DataModelType;
  nullable: boolean;
  description: string;
}

export interface DataModelTable {
  tableId: string;
  name: string;
  fields: Record<string, DataModelField>;
  linksToSingle: Record<string, LinkToSingle>;
  description: string;
}

export interface DataModel {
  version: string;
  status: string;
  tables: Record<string, DataModelTable>;
}

export interface CreateDataModelTable {
  tableName: string;
  description: string;
}

export interface CreateDataModelField {
  tableId: string;
  fieldName: string;
  description: string;
  dataType: DataModelType;
  nullable: boolean;
  isEnum: boolean;
}

export interface CreateDataModelLink {
  linkName: string;
  parentTableId: string;
  parentFieldId: string;
  childTableId: string;
  childFieldID: string;
}
