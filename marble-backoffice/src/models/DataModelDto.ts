import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type { DataModel } from "./DataModel";
import { MapObjectValues } from "@/MapUtils";

const DataModelLinkToSingleSchema = yup.object({
  linked_table_name: yup.string().defined(),
  parent_field_name: yup.string().defined(),
  child_field_name: yup.string().defined(),
});

const DataModelFieldSchema = yup.object({
  id: yup.string().required(),
  data_type: yup
    .string()
    .oneOf(["Bool", "Int", "Float", "String", "Timestamp", "unknown"])
    .required(),
  nullable: yup.boolean().required(),
  description: yup.string().defined(),
});

const DataModelTableSchema = yup.object({
  id: yup.string().required(),
  name: yup.string().required(),
  fields: yup.lazy((obj) => {
    return yup
      .object(
        MapObjectValues(obj || {}, () => {
          return DataModelFieldSchema;
        })
      )
      .nullable();
  }),
  links_to_single: yup.lazy((obj) => {
    return yup
      .object(
        MapObjectValues(obj || {}, () => {
          return DataModelLinkToSingleSchema;
        })
      )
      .nullable();
  }),
  description: yup.string().defined(),
});

const DataModelSchema = yup.object({
  version: yup.string().defined(),
  status: yup.string().defined(),
  tables: yup.lazy((obj) => {
    return yup
      .object(
        MapObjectValues(obj || {}, () => {
          return DataModelTableSchema;
        })
      )
      .nullable();
  }),
});

export function adaptDataModelApiResult(json: unknown): DataModel {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      data_model: DataModelSchema,
    })
  ).data_model;

  return {
    version: dto.version,
    status: dto.status,
    tables: MapObjectValues(
      dto.tables || {},
      (table: yup.InferType<typeof DataModelTableSchema>) => {
        return {
          tableId: table.id,
          name: table.name,
          fields: MapObjectValues(
            table.fields || {},
            (field: yup.InferType<typeof DataModelFieldSchema>) => ({
              fieldId: field.id,
              dataType: field.data_type,
              nullable: field.nullable,
              description: field.description,
            })
          ),
          linksToSingle: MapObjectValues(
            table.links_to_single || {},
            (field: yup.InferType<typeof DataModelLinkToSingleSchema>) => ({
              linkedTableName: field.linked_table_name,
              parentFieldName: field.parent_field_name,
              childFieldName: field.child_field_name,
            })
          ),
          description: table.description,
        };
      }
    ),
  };
}

export function adaptDataModelDto(model: DataModel) {
  return {
    version: model.version,
    status: model.status,
    tables: MapObjectValues(model.tables, (table) => ({
      name: table.name,
      fields: MapObjectValues(table.fields, (field) => ({
        data_type: field.dataType,
        nullable: field.nullable,
        description: field.description,
      })),
      links_to_single: MapObjectValues(table.linksToSingle, (field) => ({
        linked_table_name: field.linkedTableName,
        parent_field_name: field.parentFieldName,
        child_field_name: field.childFieldName,
      })),
      description: table.description,
    })),
  };
}
