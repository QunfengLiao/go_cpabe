#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:18080/api/v1}"
EMAIL="${EMAIL:-owner@example.com}"
PASSWORD="${PASSWORD:-Passw0rd!Demo}"

echo "1. 注册 data_owner 用户"
curl -fsS -X POST "$BASE_URL/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{
    \"email\": \"$EMAIL\",
    \"password\": \"$PASSWORD\",
    \"confirm_password\": \"$PASSWORD\",
    \"nickname\": \"数据拥有者\",
    \"role\": \"data_owner\"
  }" | tee /tmp/go_cpabe_register.json

echo
echo "2. 登录并提取 Token"
LOGIN_JSON="$(curl -fsS -X POST "$BASE_URL/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{
    \"email\": \"$EMAIL\",
    \"password\": \"$PASSWORD\"
  }")"
echo "$LOGIN_JSON" | tee /tmp/go_cpabe_login.json

ACCESS_TOKEN="$(printf '%s' "$LOGIN_JSON" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')"
REFRESH_TOKEN="$(printf '%s' "$LOGIN_JSON" | sed -n 's/.*"refresh_token":"\([^"]*\)".*/\1/p')"

if [[ -z "$ACCESS_TOKEN" || -z "$REFRESH_TOKEN" ]]; then
  echo "未能提取 Token" >&2
  exit 1
fi

echo
echo "3. 获取当前用户"
curl -fsS "$BASE_URL/users/me" \
  -H "Authorization: Bearer $ACCESS_TOKEN" | tee /tmp/go_cpabe_me.json

echo
echo "4. 刷新 Token"
curl -fsS -X POST "$BASE_URL/auth/refresh" \
  -H 'Content-Type: application/json' \
  -d "{
    \"refresh_token\": \"$REFRESH_TOKEN\"
  }" | tee /tmp/go_cpabe_refresh.json

echo
echo "5. 退出登录"
curl -fsS -X POST "$BASE_URL/auth/logout" \
  -H 'Content-Type: application/json' \
  -d "{
    \"refresh_token\": \"$REFRESH_TOKEN\"
  }" | tee /tmp/go_cpabe_logout.json

echo
echo "验证流程完成"
