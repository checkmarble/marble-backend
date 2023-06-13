import { useCallback } from "react";
import type { MarbleApi, IngestObject } from "@/infra/MarbleApi";

export function useIngestion(
  marbleApi: MarbleApi | null
) {

  const ingest = useCallback(async (): Promise<IngestObject[]> => {
    if (!marbleApi) {
      throw Error("MarbleApi not initialized");
    }

    const companyAId = "Zorg-client-a";
    const accountId = "account-a-id";

    const now = new Date().toISOString();

    const all = await Promise.all([
      // two companies
      marbleApi.ingest({
        tableName: "companies",
        content: {
          object_id: companyAId,
          updated_at: now,
          name: "Zorg-client-a",
        },
      }),
      marbleApi.ingest({
        tableName: "companies",
        content: {
          object_id: "client-a-id",
          updated_at: now,
          name: "Zorg-client-b",
        },
      }),

      // an account in companyAId
      marbleApi.ingest({
        tableName: "accounts",
        content: {
          object_id: accountId,
          updated_at: now,
          name: "Account A",
          balance: 10,
          company_id: companyAId,
          currency: "$",
          is_frozen: false,
        },
      }),

      // two transactions account in companyAId
      injectTransaction(marbleApi, accountId, "transaction_a", 12.5),
      injectTransaction(marbleApi, accountId, "transaction_b", 10),
    ])
    return all

  }, [marbleApi]);

  return {
    ingest,
  };
}

async function injectTransaction(
  api: MarbleApi,
  accountId: string,
  transaction_id: string,
  amount: number
): Promise<IngestObject> {
  const tableName = "transactions";
  const content = {
    object_id: transaction_id,
    updated_at: new Date().toISOString(),
    amount: amount,
    account_id: accountId,
    bic_country: "FR",
    country: "FR",
    description: "Transaction A",
    direction: "IN",
    status: "OK",
    title: "Transaction A",
  };
  await api.ingest({
    tableName: "transactions",
    content: content,
  });
  return {
    tableName: tableName,
    content: content,
  };
}
