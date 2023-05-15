import yup from "yup";

export function adaptDtoWithYup<Dto, Schema extends yup.Schema>(
  json: unknown,
  schema: Schema
): Dto {
  return schema.validateSync(json, {
    stripUnknown: true,
  });
}
