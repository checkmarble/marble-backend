import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import { Decision } from "./Decision";

const DecisionSchema = yup.object({
  id: yup.string().defined(),
});

export type DecisionDto = yup.InferType<typeof DecisionSchema>;

export function adaptDecision(dto: DecisionDto): Decision {
  return {
    decisionId: dto.id,
  };
}

export function adaptDecisionsApiResult(json: unknown): Decision[] {
  const dtos = adaptDtoWithYup(json, yup.array().of(DecisionSchema).required());
  return dtos.map(adaptDecision);
}

export function adaptSingleDecisionApiResult(json: unknown): Decision {
  const dto = adaptDtoWithYup(json, DecisionSchema);
  return adaptDecision(dto);
}
