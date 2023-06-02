import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

const UserSchema = yup.object({
  user_id: yup.string().required(),
  email: yup.string().required(),
  role: yup.string().required(),
  organization_id: yup.string().defined(),
});

export type UserDto = yup.InferType<typeof UserSchema>;

// ------ UsersApiResultDto

const UsersApiResultSchema = yup.object({
    users: yup.array().required().of(UserSchema),
});

export type UsersApiResultDto = yup.InferType<
  typeof UsersApiResultSchema
>;

export function adaptUsersApiResultDto(
  json: unknown
): UsersApiResultDto {
  return adaptDtoWithYup(json, UsersApiResultSchema);
}

// ------ SingleUserApiResultDto

const SingleUserApiResultSchema = yup.object({
    user: UserSchema,
});

export type SingleUserApiResultDto = yup.InferType<
  typeof SingleUserApiResultSchema
>;

export function adaptSingleUserApiResultDto(
  json: unknown
): SingleUserApiResultDto {
  return adaptDtoWithYup(json, SingleUserApiResultSchema);
}
