import { getApiBaseUrl } from "./config";

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status?: number
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const url = `${getApiBaseUrl()}${path}`;

  let response: Response;
  try {
    response = await fetch(url, init);
  } catch {
    throw new ApiError("后端服务暂时不可用");
  }

  if (!response.ok) {
    throw new ApiError("后端请求失败", response.status);
  }

  return (await response.json()) as T;
}
