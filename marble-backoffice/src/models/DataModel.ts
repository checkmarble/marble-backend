export interface LinkToSingle {
  linkedTableName: string;
  parentFieldName: string;
  childFieldName: string;
}

export interface DataModelField {
  dataType: "Bool" | "Int" | "Float" | "String" | "Timestamp" | "unknown";
  nullable: boolean;
}

export interface DataModelTable {
  name: string;
  fields: Record<string, DataModelField>;
  linksToSingle: Record<string, LinkToSingle>;
}

export interface DataModel {
  version: string;
  status: string;
  tables: Record<string, DataModelTable>;
}
