import { adaptPostTokenResult } from "@/models/marbleToken";
import {
  HttpMethod,
  fetchJson,
  HttpError,
  setAuthorizationBearerHeader,
} from "./fetchUtils";
import { AuthorizedFetcher } from "./AuthorizedFetcher";

type SetCredentialForMarbleToken = (headers: Headers) => Promise<void>;

export class MarbleApiFetcher implements AuthorizedFetcher {
  baseUrl: URL;
  setCredentialForMarbleToken: SetCredentialForMarbleToken;
  cachedMarbleToken: string | null;

  constructor(
    baseUrl: URL,
    setCredentialForMarbleToken: SetCredentialForMarbleToken
  ) {
    this.baseUrl = baseUrl;
    this.setCredentialForMarbleToken = setCredentialForMarbleToken;
    this.cachedMarbleToken = null;
  }

  async fetchMarbleToken(): Promise<string> {
    const request = new Request(new URL("/token", this.baseUrl), {
      method: HttpMethod.Post,
    });
    await this.setCredentialForMarbleToken(request.headers);

    const result = await fetchJson(request);
    return adaptPostTokenResult(result).access_token;
  }

  async getCachedMarbleToken(refresh = false): Promise<string> {
    if (!refresh && this.cachedMarbleToken !== null) {
      return this.cachedMarbleToken;
    }

    const token = await this.fetchMarbleToken();
    this.cachedMarbleToken = token;

    return token;
  }

  async authorizedApiFetch(request: Request): Promise<unknown> {
    try {
      const marbleToken = await this.getCachedMarbleToken();
      setAuthorizationBearerHeader(request.headers, marbleToken);
      return await fetchJson(request);
    } catch (error) {
      if (error instanceof HttpError) {
        if (error.statusCode === 401) {
          // 401: let's try with a refreshed token
          const refreshedMarbleToken = await this.getCachedMarbleToken(true);
          setAuthorizationBearerHeader(request.headers, refreshedMarbleToken);
          return await fetchJson(request);
        }
      }
      throw error;
    }
  }
}
