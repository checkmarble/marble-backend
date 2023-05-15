import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";

const PostTokenResultSchema = yup.object({
  access_token: yup.string().required(),
});

export type PostTokenResultDto = yup.InferType<typeof PostTokenResultSchema>;

export function adaptPostTokenResult(json: unknown): PostTokenResultDto {
  return adaptDtoWithYup(json, PostTokenResultSchema);
}
