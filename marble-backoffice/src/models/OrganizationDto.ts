import * as yup from "yup";
import { Organization } from "./Organization";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

const OrganizationSchema = yup.object({
  id: yup.string().required(),
  name: yup.string().required(),
  // date_created: yup.date().required(),
});

export type OrganizationDto = yup.InferType<typeof OrganizationSchema>;

export function adaptOrganization(dto: OrganizationDto): Organization {
  return {
    organizationId: dto.id,
    name: dto.name,
  };
}

export function adaptOrganizationsApiResult(json: unknown): Organization[] {
  const dtos = adaptDtoWithYup(
    json,
    yup.object({
      organizations: yup.array().of(OrganizationSchema).required(),
    })

  );
  return dtos.organizations.map((dto) => adaptOrganization(dto));
}

export function adaptSingleOrganizationApiResult(json: unknown): Organization {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      organization: OrganizationSchema,
    })
  );
  return adaptOrganization(dto.organization);
}
