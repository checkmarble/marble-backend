import * as yup from "yup";
import {
  type AstNode,
  type ConstantOptional,
  type ConstantType,
  NoConstant,
} from "./AstExpression";
import { MapObjectValues, ObjectToMap, MapMap, MapToObject } from "@/MapUtils";

export const ConstantOptionalSchema = yup.mixed().nullable().optional(); // {}, null or undefined

export function adaptConstantOptional(
  constant: yup.InferType<typeof ConstantOptionalSchema>
): ConstantOptional {
  return constant === undefined ? NoConstant : constant;
}

// Yup can't infer typescript type from recursive schema, let's declare it manually
export interface AstNodeDto {
  name?: string;
  constant?: ConstantType;
  children?: AstNodeDto[];
  named_children?: { [key: string]: AstNodeDto };
}

export const AstNodeSchema = yup.object({
  name: yup.string().optional(),
  constant: ConstantOptionalSchema,
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

export const AstNodeSchemaNullable =
  AstNodeSchema.nullable() as yup.Schema<AstNodeDto | null>;

export function adaptAstNode(dto: AstNodeDto): AstNode {
  return {
    name: dto.name || "",
    constant: adaptConstantOptional(dto.constant),
    children: (dto.children || []).map((child) => adaptAstNode(child)),
    namedChildren: MapMap(ObjectToMap(dto.named_children || {}), adaptAstNode),
  };
}

export function adaptAstNodeDto(model: AstNode): AstNodeDto {
  return {
    name: model.name === "" ? undefined : model.name,
    constant: model.constant === NoConstant ? undefined : model.constant,
    children: (model.children || []).map((child) => adaptAstNodeDto(child)),
    named_children: MapToObject(MapMap(model.namedChildren, adaptAstNodeDto)),
  };
}

export function adaptLitteralAstNode(json: unknown): AstNode {
  const dto = AstNodeSchema.validateSync(json);
  return adaptAstNode(dto);
}
