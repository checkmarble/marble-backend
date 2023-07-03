import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import { BuilderIdentifiers } from "./BuilderIdentifiers";

const DataAccessesIdentifierSchema = yup.object({
  var_name: yup.string().required(),
});

export function adaptBuilderIdentifiers(json: unknown): BuilderIdentifiers {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      data_accesses_identifiers: yup
        .array()
        .defined()
        .of(DataAccessesIdentifierSchema),
    })
  );

  return {
    dataAccesses: dto.data_accesses_identifiers.map((d) => {
      return {
        varName: d.var_name,
      };
    }),
  };
}
