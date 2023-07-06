import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import { type AstNode, type ConstantType, NoConstant } from "./AstExpression";
import { MapObjectValues } from "@/MapUtils";

// Yup can't infer typescript type from recursive schema, let's declare it manually
export interface AstNodeDto {
  name?: string;
  constant?: ConstantType;
  children?: AstNodeDto[];
  named_children?: { [key: string]: AstNodeDto };
}

export const AstNodeSchema = yup.object({
  name: yup.string().optional(),
  constant: yup.mixed(),
  children: yup
    .array()
    .of(yup.lazy(() => AstNodeSchema.default(null)))
    .optional(),
  named_children: yup.lazy((obj) => {
    return yup.object(
      MapObjectValues(obj || {}, () => {
        return AstNodeSchema.default(undefined);
      })
    );
  }),
}) as yup.Schema<AstNodeDto>; // Can't use lazy schema as array().of argument in TypeScript: https://github.com/jquense/yup/issues/1190

export function adaptAstNode(dto: AstNodeDto): AstNode {
  return {
    name: dto.name || "",
    constant: dto.constant === undefined ? NoConstant : dto.constant,
    children: (dto.children || []).map((child) => adaptAstNode(child)),
    namedChildren: MapObjectValues(dto.named_children || {}, adaptAstNode),
  };
}

export function adaptAstNodeDto(model: AstNode): AstNodeDto {
  return {
    name: model.name === "" ? undefined : model.name,
    constant: model.constant === NoConstant ? undefined : model.constant,
    children: (model.children || []).map((child) => adaptAstNodeDto(child)),
    named_children: MapObjectValues(model.namedChildren || {}, adaptAstNodeDto),
  };
}
export function adapAstValidateSchemaResult(json: unknown) {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      expression: AstNodeSchema,
      validation_errors: yup.array().of(yup.string().defined()).optional(),
    })
  );

  return {
    expression: adaptAstNode(dto.expression),
    validationErrors: dto.validation_errors || [],
  };
}
