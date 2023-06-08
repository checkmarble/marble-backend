export class HttpError {
  request: Request;
  response: Response;

  constructor(request: Request, response: Response) {
    this.request = request;
    this.response = response;
  }

  get statusCode(): number {
    return this.response.status;
  }
}

export enum HttpMethod {
  Post = "POST",
  Delete = "DELETE",
  Get = "GET",
}

export async function fetchJson(request: Request): Promise<unknown> {
  const response = await fetch(request);
  if (!response.ok) {
    throw new HttpError(request, response);
  }
  // test if the response content type is json
  const contentType = response.headers.get("content-type");
  if (contentType == "application/json") {
    return await response.json();
  }
  return Promise.resolve({});
}

export async function makePostRequest(args: {
  url: URL;
  body: unknown;
  headers?: Record<string, string>;
}) {
  const headers = args.headers || {};
  return new Request(args.url, {
    method: HttpMethod.Post,
    headers: {
      ...headers,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(args.body),
  });
}

export function setAuthorizationBearerHeader(headers: Headers, token: string) {
  headers.set("Authorization", `Bearer ${token}`);
}
