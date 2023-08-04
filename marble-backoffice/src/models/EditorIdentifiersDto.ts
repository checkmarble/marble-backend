import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type { EditorIdentifiers, Identifier } from "./EditorIdentifiers";
import { AstNodeSchema, adaptAstNode } from "./AstExpressionDto";

const IdentifierSchema = yup.object({
  name: yup.string().required(),
  description: yup.string().defined(),
  node: AstNodeSchema,
});

export type IdentifierDto = yup.InferType<typeof IdentifierSchema>;

function adaptIdentigiers(dto: IdentifierDto): Identifier {
  return {
    name: dto.name,
    description: dto.description,
    node: adaptAstNode(dto.node),
  };
}

export function adaptEditorIdentifiers(json: unknown): EditorIdentifiers {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      database_accessors: yup.array().defined().of(IdentifierSchema),
      payload_accessors: yup.array().defined().of(IdentifierSchema),
      custom_list_accessors: yup.array().defined().of(IdentifierSchema),
    })
  );

  return {
    databaseAccessors: dto.database_accessors.map(adaptIdentigiers),
    payloadAccessors: dto.payload_accessors.map(adaptIdentigiers),
    customListAccessors: dto.custom_list_accessors.map(adaptIdentigiers),
  };
}
