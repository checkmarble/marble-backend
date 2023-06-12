import yup from "yup";

export function adaptDtoWithYup<Schema extends yup.Schema>(
  json: unknown,
  schema: Schema
): yup.InferType<Schema> {
  return schema.validateSync(json, {
    stripUnknown: true,
  });
}
