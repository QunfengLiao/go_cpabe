import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Descriptions, Dropdown, message } from "antd";
import { MoreOutlined, PlusOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useAuth } from "../auth/AuthContext";
import { listMyRSAKeys, registerMyRSAKey, type RSAPublicKey } from "../api/encryption";
import { ContentCard, DataTable, DetailDrawer, FingerprintCell, PageHeader, PageShell, StatusBadge, SummaryStat } from "../components/ui";

type LocalRSAKeySummary = {
  fingerprintSha256: string;
  publicKeyId?: string;
  version?: number;
  registrationPending: boolean;
  createdAt: string;
};

/** MyRSAKeysPage 展示公钥登记状态和本地私钥摘要，私钥正文与绝对路径始终留在主进程。 */
export function MyRSAKeysPage() {
  const auth = useAuth();
  const [keys, setKeys] = useState<RSAPublicKey[]>([]);
  const [localKeys, setLocalKeys] = useState<LocalRSAKeySummary[]>([]);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState("");
  const [selectedKey, setSelectedKey] = useState<RSAPublicKey | null>(null);

  async function load() {
    setLoading(true);
    try {
      const [serverKeys, storedKeys] = await Promise.all([
        listMyRSAKeys(),
        window.desktopEncryption?.listLocalRSAKeys(auth.currentUserId, auth.currentTenantId) ?? Promise.resolve([])
      ]);
      setKeys(serverKeys);
      setLocalKeys(storedKeys);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "密钥列表加载失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, [auth.currentUserId, auth.currentTenantId]);

  async function generateOrResume() {
    if (!window.desktopEncryption) return;
    setBusy(true); setNotice("");
    try {
      const pending = await window.desktopEncryption.listPendingRSAKeys(auth.currentUserId, auth.currentTenantId);
      const materials = pending.length > 0 ? pending : [await window.desktopEncryption.generateRSAKey(auth.currentUserId, auth.currentTenantId)];
      for (const material of materials) {
        const registered = await registerMyRSAKey(material);
        await window.desktopEncryption.markRSAKeyRegistered(auth.currentUserId, auth.currentTenantId, material.fingerprintSha256, registered.id, registered.version);
      }
      message.success(pending.length > 0 ? "待登记公钥已恢复登记" : "RSA 私钥已由本地安全存储保护，公钥登记成功");
      await load();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "密钥生成或登记失败");
    } finally {
      setBusy(false);
    }
  }

  async function importPrivateKey() {
    if (!window.desktopEncryption) return;
    setBusy(true); setNotice("");
    try {
      const material = await window.desktopEncryption.importPrivateKey(auth.currentUserId, auth.currentTenantId);
      if (!material) return;
      const registered = await registerMyRSAKey(material);
      await window.desktopEncryption.markRSAKeyRegistered(auth.currentUserId, auth.currentTenantId, material.fingerprintSha256, registered.id, registered.version);
      message.success("RSA 私钥已安全导入，公钥登记成功");
      await load();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "RSA 私钥导入失败");
    } finally {
      setBusy(false);
    }
  }

  async function openPrivateKeyDirectory() {
    if (!window.desktopEncryption) return;
    const opened = await window.desktopEncryption.openRSAKeyDirectory(auth.currentUserId, auth.currentTenantId);
    if (opened) message.success("已打开本地私钥文件目录");
    else message.info("当前账号还没有可打开的本地私钥目录");
  }

  const localByFingerprint = useMemo(() => new Map(localKeys.map((key) => [key.fingerprintSha256, key])), [localKeys]);
  const currentKey = useMemo(() => [...keys].filter((key) => key.status === "ACTIVE").sort((left, right) => right.version - left.version)[0] ?? keys[0], [keys]);
  const registeredLocalCount = localKeys.filter((key) => !key.registrationPending).length;

  return (
    <PageShell className="rsa-keys-page">
      <PageHeader
        title="我的密钥"
        description="RSA 私钥只保存在本地安全存储中；服务端仅保存用于封装文件密钥的公钥。"
        actions={<Dropdown menu={{ items: [
          { key: "export-public", label: "导出公钥", disabled: true },
          { key: "export-private", label: "导出加密私钥备份", disabled: true },
          { type: "divider" },
          { key: "directory", label: "打开本地密钥目录", onClick: () => void openPrivateKeyDirectory() },
          { key: "refresh", label: "刷新密钥列表", onClick: () => void load() },
          { key: "security", label: "查看安全说明", onClick: () => message.info("私钥不会通过 IPC 返回 Renderer，也不会上传后端。") }
        ] }}>
          <Button icon={<MoreOutlined />}>更多操作</Button>
        </Dropdown>}
      />
      {notice && <Alert type="error" showIcon closable message={notice} onClose={() => setNotice("")} />}

      <div className="rsa-summary-grid">
        <ContentCard className="summary-stat-card"><SummaryStat label="当前使用密钥" value={currentKey ? `v${currentKey.version}` : "未配置"} description={currentKey ? shortFingerprint(currentKey.fingerprint_sha256) : "请先生成或导入密钥"} tone="primary" /></ContentCard>
        <ContentCard className="summary-stat-card"><SummaryStat label="本地私钥" value={registeredLocalCount > 0 ? `${registeredLocalCount} 个可用` : "未发现"} description="Electron 本地安全存储" tone="success" /></ContentCard>
        <ContentCard className="summary-stat-card"><SummaryStat label="服务端公钥" value={`${keys.length} 个`} description="用于生成 RSA-OAEP 密钥信封" /></ContentCard>
      </div>

      <ContentCard className="rsa-operation-card">
        <div>
          <h3>密钥操作</h3>
          <p>生成新密钥或导入已有私钥，完成后公钥会登记到当前租户。</p>
        </div>
        <div className="key-operation-actions">
          <Button type="primary" icon={<PlusOutlined />} loading={busy} disabled={!window.desktopEncryption} onClick={() => void generateOrResume()}>生成新密钥</Button>
          <Button icon={<SafetyCertificateOutlined />} loading={busy} disabled={!window.desktopEncryption} onClick={() => void importPrivateKey()}>导入已有私钥</Button>
        </div>
      </ContentCard>

      <ContentCard className="rsa-key-list-card" title="密钥列表" extra={<span className="content-card-meta">共 {keys.length} 个</span>}>
        <DataTable rowKey="id" loading={loading} dataSource={keys} pagination={false} locale={{ emptyText: "暂无已登记公钥" }} columns={[
          { title: "版本", dataIndex: "version", render: (value: number, key: RSAPublicKey) => <div className="key-version-cell">{key.id === currentKey?.id && <StatusBadge label="当前使用" tone="info" />}<span>v{value}</span></div> },
          { title: "算法", responsive: ["md"], render: () => <div className="algorithm-display"><strong>RSA-OAEP</strong><span>3072 位 · SHA-256</span></div> },
          { title: "公钥指纹", dataIndex: "fingerprint_sha256", render: (value: string) => <FingerprintCell value={value} /> },
          { title: "本地私钥", dataIndex: "fingerprint_sha256", responsive: ["md"], render: (value: string) => localByFingerprint.has(value) ? <StatusBadge label="本地可用" tone="success" /> : <StatusBadge label="本地缺失" /> },
          { title: "服务端登记", dataIndex: "status", responsive: ["sm"], render: (value: string) => <StatusBadge label={serverStatus(value)} tone={value === "ACTIVE" ? "success" : value === "DISABLED" ? "warning" : "neutral"} /> },
          { title: "创建时间", dataIndex: "created_at", responsive: ["lg"], render: (value: string) => <span className="nowrap-cell">{value?.replace("T", " ").slice(0, 16) || "暂无"}</span> },
          { title: "操作", key: "actions", render: (_value: unknown, key: RSAPublicKey) => <Button type="link" onClick={() => setSelectedKey(key)}>详情</Button> }
        ]} />
      </ContentCard>

      <DetailDrawer title="密钥详情" open={Boolean(selectedKey)} width={560} onClose={() => setSelectedKey(null)}>
        {selectedKey && <div className="detail-stack">
          <div className="key-detail-hero"><div><h3>RSA 密钥 v{selectedKey.version}</h3><StatusBadge label={serverStatus(selectedKey.status)} tone={selectedKey.status === "ACTIVE" ? "success" : "neutral"} /></div></div>
          <Descriptions column={1} size="small" items={[
            { key: "id", label: "Key ID", children: selectedKey.id },
            { key: "version", label: "版本", children: `v${selectedKey.version}` },
            { key: "bits", label: "RSA 位数", children: `${selectedKey.key_bits} 位` },
            { key: "oaep", label: "OAEP 哈希", children: "SHA-256" },
            { key: "algorithm", label: "算法", children: "RSA-OAEP-SHA256 / RSA 3072" },
            { key: "fingerprint", label: "完整指纹", children: <FingerprintCell value={selectedKey.fingerprint_sha256} /> },
            { key: "local", label: "本地私钥", children: localByFingerprint.has(selectedKey.fingerprint_sha256) ? <StatusBadge label="本地可用" tone="success" /> : <StatusBadge label="本地缺失" /> },
            { key: "server", label: "服务端公钥", children: <StatusBadge label={serverStatus(selectedKey.status)} tone={selectedKey.status === "ACTIVE" ? "success" : "neutral"} /> },
            { key: "default", label: "默认密钥", children: selectedKey.id === currentKey?.id ? "是" : "否" },
            { key: "envelopes", label: "关联密钥信封数量", children: "暂无记录" },
            { key: "recent", label: "最近使用记录", children: "暂无记录" },
            { key: "created", label: "创建时间", children: selectedKey.created_at?.replace("T", " ").slice(0, 19) || "暂无" }
          ]} />
          <Alert type="info" showIcon message="安全边界" description="私钥只在 Electron 主进程中解封和使用。Renderer 不会得到私钥明文，Go 后端也不会保存私钥。" />
        </div>}
      </DetailDrawer>
    </PageShell>
  );
}

function shortFingerprint(value: string) {
  return value ? `${value.slice(0, 12)}…${value.slice(-8)}` : "未知";
}

function serverStatus(status: string) {
  if (status === "ACTIVE") return "已登记";
  if (status === "REVOKED") return "已吊销";
  if (status === "DISABLED") return "已停用";
  return "历史版本";
}
