export interface LinkToSingle {
  linkedTableName: string;
  parentFieldName: string;
  childFieldName: string;
}

export interface DataModelField {
  dataType: "Bool" | "Int" | "Float" | "String" | "Timestamp" | "unknown";
  nullable: boolean;
  description: string;
}

export interface DataModelTable {
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
