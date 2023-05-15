import { adaptPostTokenResult } from "@/models/marbleToken";
import {
  HttpMethod,
  fetchJson,
  HttpError,
  setAuthorizationBearerHeader,
} from "./fetchUtils";

export class MarbleApi {
  fetchFirebaseIdToken: () => Promise<string>;
  baseUrl: URL;
  cachedMarbleToken: string | null;

  constructor(baseUrl: URL, fetchFirebaseIdToken: () => Promise<string>) {
    this.baseUrl = baseUrl;
    this.fetchFirebaseIdToken = fetchFirebaseIdToken;
    this.cachedMarbleToken = null;
  }

  apiUrl(path: string): URL {
    return new URL(path, this.baseUrl);
  }

  async fetchMarbleToken() : Promise<string> {
    const firebaseIdToken = await this.fetchFirebaseIdToken();
    const request = new Request(this.apiUrl("/token"), {
      method: HttpMethod.Post,
    });
    setAuthorizationBearerHeader(request.headers, firebaseIdToken);
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

  async getAuthorizedJson(path: string): Promise<unknown> {
    const request = new Request(this.apiUrl(path), {
      method: HttpMethod.Get,
    });

    return this.authorizedApiFetch(request);
  }

  async organizations(): Promise<unknown> {
    return this.getAuthorizedJson("/organization");
  }
}
