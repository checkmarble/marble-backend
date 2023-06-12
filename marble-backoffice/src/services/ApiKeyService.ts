import { useCallback, useEffect, useState } from "react";
import { type OrganizationRepository, fetchApiKeys } from "@/repositories";
import { Role } from "@/models";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { type LoadingDispatcher } from "@/hooks/Loading";
import { type MarbleApi } from "@/infra/MarbleApi";

export interface ApiKeyService {
  organizationRepository: OrganizationRepository;
  marbleApiWithApiKeyFactory: (apiKey: string) => MarbleApi;
}

export function useClientRoleApiKey(
  service: ApiKeyService,
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

export function useMarbleApiWithClientRoleApiKey(
  service: ApiKeyService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const { apiKey } = useClientRoleApiKey(
    service,
    loadingDispatcher,
    organizationId
  );

  const [marbleApiWithClientRoleApiKey, setMarbleApi] = useState<MarbleApi | null>(null);
  useEffect(() => {
    if (apiKey !== null) {
      setMarbleApi(service.marbleApiWithApiKeyFactory(apiKey));
    }
  }, [apiKey, service]);

  return {
    apiKey,
    marbleApiWithClientRoleApiKey
  }
}
