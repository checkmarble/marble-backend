import { useCallback, useEffect, useState } from "react";
import { Role } from "@/models";
import { type OrganizationRepository, fetchApiKeys } from "@/repositories";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { type LoadingDispatcher } from "@/hooks/Loading";
import { MarbleApi } from "@/infra/MarbleApi";

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
  apiKey: string
) {
  const [marbleApi, setMarbleApi] = useState<MarbleApi | null>(null);

  useEffect(() => {
    if (apiKey) {
      setMarbleApi(service.marbleApiWithApiKeyFactory(apiKey));
    }
  }, [apiKey, service]);

  const ingest = useCallback(() => {
    if (!marbleApi) {
      throw Error("MarbleApi not initialized");
    }
  }, [marbleApi]);

  return {
    ingest,
  };
}
