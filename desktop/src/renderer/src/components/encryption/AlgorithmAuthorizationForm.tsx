import { Alert, Avatar, Badge, Card, Checkbox, Empty, Input, Select, Tag, Typography } from "antd";
import { KeyOutlined, SearchOutlined, UserOutlined } from "@ant-design/icons";
import { useMemo, useState } from "react";
import type { AlgorithmCapability, RSAPublicKey, RSARecipient } from "../../api/encryption";
import { shortHash } from "../../utils/fileDisplay";

const authorizationFormRegistry = new Map<string, "rsa">([["RSA_RECIPIENT", "rsa"], ["RSA_RECIPIENTS", "rsa"]]);

export function authorizationFormKind(authorizationType?: string): "rsa" | "unsupported" {
  if (!authorizationType) return "unsupported";
  return authorizationFormRegistry.get(authorizationType) ?? "unsupported";
}

export function AlgorithmAuthorizationForm({
  algorithms,
  algorithmCode,
  recipients,
  recipientIds,
  keysByUser,
  keyIdsByUser,
  ownerUserId,
  disabled,
  onAlgorithm,
  onRecipients,
  onKey
}: {
  algorithms: AlgorithmCapability[];
  algorithmCode: string;
  recipients: RSARecipient[];
  recipientIds: number[];
  keysByUser: Record<number, RSAPublicKey[]>;
  keyIdsByUser: Record<number, string>;
  ownerUserId?: number;
  disabled?: boolean;
  onAlgorithm: (value: string) => void;
  onRecipients: (value: number[]) => void;
  onKey: (userId: number, value: string) => void;
}) {
  const [keyword, setKeyword] = useState("");
  const algorithm = algorithms.find((item) => item.code === algorithmCode);
  const formKind = authorizationFormKind(algorithm?.authorization_type);
  const selectedCount = recipientIds.length;
  const filteredRecipients = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) return recipients;
    return recipients.filter((recipient) => `${recipient.display_name} ${recipient.email ?? ""} ${recipient.role ?? ""}`.toLowerCase().includes(normalized));
  }, [keyword, recipients]);

  function toggleRecipient(userId: number, checked: boolean) {
    if (ownerUserId === userId && !checked) return;
    if (checked) onRecipients(Array.from(new Set([...recipientIds, userId])));
    else onRecipients(recipientIds.filter((id) => id !== userId));
  }

  return (
    <Card title="2. 选择算法和接收者" className="encryption-card">
      <div className="algorithm-form-stack">
        <div className="algorithm-summary">
          <Select
            className="algorithm-select"
            placeholder="选择算法"
            value={algorithmCode || undefined}
            disabled={disabled}
            options={algorithms.map((item) => ({ value: item.code, label: `${item.display_name} / v${item.version}`, disabled: item.enabled === false }))}
            onChange={onAlgorithm}
          />
          <div>
            <strong>{algorithm?.display_name ?? "请选择算法"}</strong>
            <span>AES-256-GCM 加密文件内容，DEK 使用 {algorithm?.code ?? "所选算法"} 保护。</span>
          </div>
        </div>

        {formKind === "rsa" ? (
          <div className="recipient-selector">
            <div className="recipient-selector-toolbar">
              <Input prefix={<SearchOutlined />} allowClear placeholder="搜索接收者姓名、账号或角色" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
              <Badge count={selectedCount} overflowCount={999} showZero>
                <Tag color={selectedCount > 0 ? "blue" : "default"}>已选择接收者</Tag>
              </Badge>
            </div>
            <div className="selected-recipient-tags">
              {recipientIds.map((id) => {
                const recipient = recipients.find((item) => item.user_id === id);
                return <Tag key={id} color={ownerUserId === id ? "blue" : "default"}>{recipient?.display_name ?? "未知用户"}{ownerUserId === id ? " · 文件拥有者" : ""}</Tag>;
              })}
              {recipientIds.length === 0 && <Typography.Text type="secondary">请选择至少一位拥有有效公钥的接收者。</Typography.Text>}
            </div>
            <div className="recipient-list">
              {filteredRecipients.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前租户暂无可用接收者" /> : filteredRecipients.map((recipient) => {
                const keys = keysByUser[recipient.user_id] ?? [];
                const checked = recipientIds.includes(recipient.user_id);
                const noKey = !recipient.available || recipient.active_key_count === 0;
                const lockedOwner = ownerUserId === recipient.user_id;
                return (
                  <div className={`recipient-row${checked ? " recipient-row-selected" : ""}${noKey ? " recipient-row-disabled" : ""}`} key={recipient.user_id}>
                    <Checkbox checked={checked} disabled={disabled || noKey || lockedOwner} onChange={(event) => toggleRecipient(recipient.user_id, event.target.checked)} />
                    <Avatar icon={<UserOutlined />}>{recipient.display_name.slice(0, 1)}</Avatar>
                    <div className="recipient-row-main">
                      <div className="recipient-row-title">
                        <strong>{recipient.display_name}</strong>
                        {lockedOwner && <Tag color="blue">文件拥有者</Tag>}
                        {noKey && <Tag color="orange">暂无有效公钥</Tag>}
                      </div>
                      <span>{recipient.email || recipient.role || "账号信息缺失"} · {recipient.active_key_count} 个有效公钥</span>
                    </div>
                    <Select
                      className="recipient-key-select"
                      placeholder="公钥版本"
                      value={keyIdsByUser[recipient.user_id] || undefined}
                      disabled={disabled || !checked || keys.length === 0}
                      options={keys.map((key) => ({ value: key.id, label: `v${key.version} · ${shortHash(key.fingerprint_sha256)}` }))}
                      onChange={(value) => onKey(recipient.user_id, value)}
                      suffixIcon={<KeyOutlined />}
                    />
                  </div>
                );
              })}
            </div>
          </div>
        ) : algorithm ? <Alert type="warning" showIcon message="该授权类型尚未在本期客户端实现" /> : null}
      </div>
    </Card>
  );
}
