import * as yup from "yup";
import { type MarbleApi } from "@/infra/MarbleApi";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type {
  CreateDataModelField,
  CreateDataModelLink,
  CreateDataModelTable,
} from "@/models";

export interface DataModelRepository {
  marbleApi: MarbleApi;
}

export async function createDataModelTable(
  repository: DataModelRepository,
  organizationId: string,
  createDataModelTable: CreateDataModelTable
): Promise<string> {
  const json = await repository.marbleApi.postDataModelTable(
    organizationId,
    createDataModelTable
  );

  return adaptDtoWithYup(
    json,
    yup.object({
      id: yup.string().required(),
    })
  ).id;
}

export async function createDataModelField(
  repository: DataModelRepository,
  organizationId: string,
  createDataModelField: CreateDataModelField
): Promise<string> {
  const json = await repository.marbleApi.postDataModelField(
    organizationId,
    createDataModelField
  );

  return adaptDtoWithYup(
    json,
    yup.object({
      id: yup.string().required(),
    })
  ).id;
}

export async function createDataModelLink(
  repository: DataModelRepository,
  organizationId: string,
  createDataModelLink: CreateDataModelLink
) {
  await repository.marbleApi.postDataModelLink(
    organizationId,
    createDataModelLink
  );
}
