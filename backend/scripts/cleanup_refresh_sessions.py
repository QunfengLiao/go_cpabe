#!/usr/bin/env python3
"""清理开发环境中无 TTL、格式异常或孤儿索引的 Refresh Session Redis Key。"""

import json
import os
import sys
import time

try:
    import redis
except ImportError:
    print("缺少 redis Python 包，请先安装 redis-py。", file=sys.stderr)
    sys.exit(2)


def main() -> int:
    client = redis.Redis(
        host=os.getenv("REDIS_HOST", "127.0.0.1"),
        port=int(os.getenv("REDIS_PORT", "6379")),
        db=int(os.getenv("REDIS_DB", "0")),
        password=os.getenv("REDIS_PASSWORD") or None,
        decode_responses=True,
    )
    dry_run = os.getenv("DRY_RUN", "true").lower() != "false"
    now = time.time()
    removed_refresh = 0
    removed_index = 0
    scanned_refresh = 0
    scanned_index = 0
    for key in client.scan_iter("auth:refresh:*"):
        scanned_refresh += 1
        ttl = client.ttl(key)
        raw = client.get(key)
        remove_reason = ""
        if ttl == -1:
            remove_reason = "missing_ttl"
        elif not raw:
            remove_reason = "empty_value"
        else:
            try:
                payload = json.loads(raw)
                expires_at = payload.get("expires_at")
                if isinstance(expires_at, str):
                    # Go 的 time.Time JSON 通常是 RFC3339；开发脚本只兜底明显无 TTL 的历史数据。
                    expires_at = None
                if isinstance(expires_at, (int, float)) and expires_at <= now:
                    remove_reason = "expired_payload"
            except json.JSONDecodeError:
                remove_reason = "invalid_json"
        if remove_reason:
            removed_refresh += 1
            print(f"{'DRY-RUN ' if dry_run else ''}delete {key}: {remove_reason}")
            if not dry_run:
                client.delete(key)
    for key in client.scan_iter("auth:user_session:*"):
        scanned_index += 1
        ttl = client.ttl(key)
        token_id = client.get(key)
        remove_reason = ""
        if ttl == -1:
            remove_reason = "missing_ttl"
        elif not token_id:
            remove_reason = "empty_value"
        elif not client.exists(f"auth:refresh:{token_id}"):
            remove_reason = "orphan_index"
        if remove_reason:
            removed_index += 1
            print(f"{'DRY-RUN ' if dry_run else ''}delete {key}: {remove_reason}")
            if not dry_run:
                client.delete(key)
    print(
        " ".join(
            [
                f"refresh_scanned={scanned_refresh}",
                f"refresh_removable={removed_refresh}",
                f"index_scanned={scanned_index}",
                f"index_removable={removed_index}",
                f"dry_run={dry_run}",
            ]
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
