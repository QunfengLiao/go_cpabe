import type { WebContents } from "electron";
import type { EncryptionProgressEvent } from "./types";

export class ProgressBridge {
  private readonly latest = new Map<string, EncryptionProgressEvent>();
  private readonly startedAt = new Map<string, number>();
  private readonly stageStartedAt = new Map<string, { stage: string; at: number }>();

  publish(target: WebContents, event: EncryptionProgressEvent): void {
    const previous = this.latest.get(event.executionId);
    if (previous && (previous.accountId !== event.accountId || previous.tenantId !== event.tenantId || isRegressive(previous, event))) return;
    const now = performance.now();
    const startedAt = this.startedAt.get(event.executionId) ?? now;
    this.startedAt.set(event.executionId, startedAt);
    const priorStage = this.stageStartedAt.get(event.executionId);
    const stageStartedAt = priorStage?.stage === event.stage ? priorStage.at : now;
    this.stageStartedAt.set(event.executionId, { stage: event.stage, at: stageStartedAt });
    const normalized = normalizeProgress(event, previous, now - startedAt, now - stageStartedAt);
    this.latest.set(event.executionId, normalized);
    if (!target.isDestroyed()) target.send("encryption:progress", normalized);
  }

  finish(executionId: string): void {
    this.latest.delete(executionId);
    this.startedAt.delete(executionId);
    this.stageStartedAt.delete(executionId);
  }

  clear(): void {
    this.latest.clear();
    this.startedAt.clear();
    this.stageStartedAt.clear();
  }
}

function isRegressive(previous: EncryptionProgressEvent, next: EncryptionProgressEvent): boolean {
  if (stageRank(next.stage) < stageRank(previous.stage)) return true;
  if (previous.stage !== next.stage) return false;
  return next.processedBytes < previous.processedBytes;
}

function normalizeProgress(event: EncryptionProgressEvent, previous: EncryptionProgressEvent | undefined, totalElapsedMs: number, stageElapsedMs: number): EncryptionProgressEvent {
  const ratio = event.totalBytes > 0 ? Math.min(1, Math.max(0, event.processedBytes / event.totalBytes)) : 0;
  const [base, span] = stageRange(event.stage);
  const calculated = event.stage === "COMPLETED" ? 100 : base + ratio * span;
  const percent = Math.max(previous?.percent ?? 0, Math.min(100, Math.round(calculated)));
  return {
    ...event,
    ...(event.stage === "UPLOADING" ? { uploadedBytes: event.processedBytes, ciphertextBytes: event.totalBytes } : {}),
    ...(event.stage === "PROTECTING_KEY" ? { protectedRecipients: event.processedBytes, totalRecipients: event.totalBytes } : {}),
    stageElapsedMs: Math.max(0, Math.round(event.stageElapsedMs ?? stageElapsedMs)),
    totalElapsedMs: Math.max(0, Math.round(event.totalElapsedMs ?? totalElapsedMs)),
    percent
  };
}

function stageRange(stage: EncryptionProgressEvent["stage"]): [number, number] {
  switch (stage) {
    case "VALIDATING": return [0, 5];
    case "ENCRYPTING_FILE": return [5, 50];
    case "PROTECTING_KEY": return [55, 15];
    case "UPLOADING": return [70, 25];
    case "SAVING_METADATA": return [95, 0];
    case "COMPLETED": return [100, 0];
    case "FAILED":
    case "CANCELLED": return [0, 0];
    default: return [0, 0];
  }
}

function stageRank(stage: EncryptionProgressEvent["stage"]): number {
  const ranks: Partial<Record<EncryptionProgressEvent["stage"], number>> = {
    PENDING: 0, VALIDATING: 1, ENCRYPTING_FILE: 2, PROTECTING_KEY: 3, UPLOADING: 4,
    SAVING_METADATA: 5, COMPLETED: 6, FAILED: 7, CANCELLED: 7
  };
  return ranks[stage] ?? 0;
}
