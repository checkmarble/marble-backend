import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import { EditorIdentifiers } from "./EditorIdentifiers";
import {
  AstNodeSchema,
  adaptAstNode,
} from "./AstExpressionDto";

export function adaptEditorIdentifiers(json: unknown): EditorIdentifiers {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      data_accessors: yup.array().defined().of(AstNodeSchema)
    })
  );

  return {
    dataAccessors: dto.data_accessors.map(adaptAstNode),
  };
}
