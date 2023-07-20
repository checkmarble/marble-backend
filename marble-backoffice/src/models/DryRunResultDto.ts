import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import {
  ConstantOptionalSchema,
  adaptConstantOptional,
} from "./AstExpressionDto";
import { DryRunResult } from "./DryRunResult";

export function adaptDryRunResult(json: unknown): DryRunResult {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      result: ConstantOptionalSchema,
      runtime_error: yup.string().defined(),
    })
  );

  return {
    result: adaptConstantOptional(dto.result),
    runtimeError: dto.runtime_error,
  };
}
