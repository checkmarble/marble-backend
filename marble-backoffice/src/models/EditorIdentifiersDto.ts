import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type { EditorIdentifiers } from "./EditorIdentifiers";
import { AstNodeSchema, adaptAstNode } from "./AstExpressionDto";

export function adaptEditorIdentifiers(json: unknown): EditorIdentifiers {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      database_accessors: yup.array().defined().of(AstNodeSchema),
      payload_accessors: yup.array().defined().of(AstNodeSchema),
    })
  );

  return {
    databaseAccessors: dto.database_accessors.map(adaptAstNode),
    payloadAccessors: dto.payload_accessors.map(adaptAstNode),
  };
}
