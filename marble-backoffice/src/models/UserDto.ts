import * as yup from "yup";
import { User } from "./User";
import { adaptRole } from "./Role";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

const UserSchema = yup.object({
  user_id: yup.string().required(),
  email: yup.string().required(),
  role: yup.string().required(),
  organization_id: yup.string().defined(),
});

export type UserDto = yup.InferType<typeof UserSchema>;

export function adaptUser(dto: UserDto): User {
  return {
    userId: dto.user_id,
    email: dto.email,
    role: adaptRole(dto.role),
    organizationId: dto.organization_id,
  };
}

export function adaptUsersApiResult(json: unknown): User[] {
  return adaptDtoWithYup(
    json,
    yup.object({
      users: yup.array().required().of(UserSchema),
    })
  ).users.map((dto) => adaptUser(dto));
}

export function adaptSingleUserApiResult(json: unknown): User {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      user: UserSchema,
    })
  ).user;
  return adaptUser(dto);
}
