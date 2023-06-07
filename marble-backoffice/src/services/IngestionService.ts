import { useCallback, useEffect, useState } from "react";
import { Role } from "@/models";
import { type OrganizationRepository, fetchApiKeys } from "@/repositories";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { type LoadingDispatcher } from "@/hooks/Loading";
import { MarbleApi, type IngestObject } from "@/infra/MarbleApi";

export interface IngestionService {
  organizationRepository: OrganizationRepository;
  marbleApiWithApiKeyFactory: (apiKey: string) => MarbleApi;
}

export function useClientRoleApiKey(
  service: IngestionService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadApiKeyWithApiClientRole = useCallback(async () => {
    const allApiKeys = await fetchApiKeys(
      service.organizationRepository,
      organizationId
    );
    const apiKey = allApiKeys.find((apikey) => apikey.role === Role.API_CLIENT);
    if (apiKey === undefined) {
      throw Error("The client has no Api key");
    }
    return apiKey.key;
  }, [service, organizationId]);

  const [apiKey] = useSimpleLoader<string>(
    loadingDispatcher,
    loadApiKeyWithApiClientRole
  );

  return {
    apiKey,
  };
}

export function useIngestion(
  service: IngestionService,
  // loadingDispatcher: LoadingDispatcher,
  apiKey: string | null
) {
  const [marbleApi, setMarbleApi] = useState<MarbleApi | null>(null);

  useEffect(() => {
    if (apiKey !== null) {
      setMarbleApi(service.marbleApiWithApiKeyFactory(apiKey));
    }
  }, [apiKey, service]);

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
      injectTransaction(marbleApi, accountId, 12.5),
      injectTransaction(marbleApi, accountId, 10),
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
  amount: number
): Promise<IngestObject> {
  const tableName = "transactions";
  const content = {
    object_id: "transaction_a+",
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
