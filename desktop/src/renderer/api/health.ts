import { request } from "./client";

export type DependencyStatus = "ok" | "error";
export type OverallStatus = "ok" | "degraded";

export interface HealthResponse {
  status: OverallStatus;
  checkedAt: string;
  app: {
    status: "ok";
    env: string;
  };
  mysql: {
    status: DependencyStatus;
    message: string;
  };
  redis: {
    status: DependencyStatus;
    message: string;
  };
}

export function getHealth(): Promise<HealthResponse> {
  return request<HealthResponse>("/health");
}
