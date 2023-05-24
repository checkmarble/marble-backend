import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

// ------ OrganizationDto

const OrganizationSchema = yup.object({
  id: yup.string().required(),
  name: yup.string().required(),
  // date_created: yup.date().required(),
});

export type OrganizationDto = yup.InferType<typeof OrganizationSchema>;

// ------ OrganizationsApiResultDto

const OrganizationsApiResultSchema = yup.object({
  organizations: yup.array().of(OrganizationSchema).required(),
});

export type OrganizationsApiResultDto = yup.InferType<
  typeof OrganizationsApiResultSchema
>;

export function adaptOrganizationsApiResultDto(
  json: unknown
): OrganizationsApiResultDto {
  return adaptDtoWithYup(json, OrganizationsApiResultSchema);
}

// ------ SingleOrganizationApiResult

const SingleOrganizationApiSchema = yup.object({
  organization: OrganizationSchema,
});

export type SingleOrganizationApiResultDto = yup.InferType<
  typeof SingleOrganizationApiSchema
>;

export function adaptSingleOrganizationApiResultDto(
  json: unknown
): SingleOrganizationApiResultDto {
  return adaptDtoWithYup(json, SingleOrganizationApiSchema);
}
