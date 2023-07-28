import { ConstantOptional, NoConstant } from "@/models";

export function AstConstantComponent({ constant }: { constant: ConstantOptional }) {
    if (constant === NoConstant) {
      return <>!No Constant!</>;
    }
    if (constant === null) {
      return <>NULL</>;
    }
    return <code>{JSON.stringify(constant)}</code>;
  }
  