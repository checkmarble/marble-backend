
export interface AuthorizedFetcher {
  authorizedApiFetch(request: Request): Promise<unknown>;
}
