import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type { DataModel } from "./DataModel";
import { MapObjectValues } from "@/MapUtils";

const DataModelLinkToSingle = yup.object({
  linked_table_name: yup.string().defined(),
  parent_field_name: yup.string().defined(),
  child_field_name: yup.string().defined(),
});

const DataModelFieldSchema = yup.object({
  data_type: yup
    .string()
    .oneOf(["Bool", "Int", "Float", "String", "Timestamp", "unknown"])
    .required(),
  nullable: yup.boolean().required(),
});

const DataModelTableSchema = yup.object({
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
          return DataModelLinkToSingle;
        })
      )
      .nullable();
  }),
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
          name: table.name,
          fields: MapObjectValues(
            table.fields || {},
            (field: yup.InferType<typeof DataModelFieldSchema>) => ({
              dataType: field.data_type,
              nullable: field.nullable,
            })
          ),
          linksToSingle: MapObjectValues(
            table.links_to_single || {},
            (field: yup.InferType<typeof DataModelLinkToSingle>) => ({
              linkedTableName: field.linked_table_name,
              parentFieldName: field.parent_field_name,
              childFieldName: field.child_field_name,
            })
          ),
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
      })),
      links_to_single: MapObjectValues(table.linksToSingle, (field) => ({
        linked_table_name: field.linkedTableName,
        parent_field_name: field.parentFieldName,
        child_field_name: field.childFieldName,
      })),
    })),
  };
}
