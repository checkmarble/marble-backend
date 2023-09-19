
export interface AuthorizedFetcher {
  authorizedJson(request: Request): Promise<unknown>;
  authorizedBlob(request: Request): Promise<Blob>;
}
