import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Input, Select, message } from "antd";
import { PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import { useNavigate, useSearchParams } from "react-router-dom";
import { API_BASE_URL } from "../api/request";
import { getFileCenterDetail, listFileCenterItems, type EncryptedFileRecord, type FileCenterScope } from "../api/encryption";
import { useAuth } from "../auth/AuthContext";
import { FileTable } from "../components/encryption/FileTable";
import { EncryptedFileDetail } from "../components/encryption/EncryptedFileDetail";
import { ContentCard, FilterToolbar, LoadingSkeleton, PageHeader, PageShell, PageTabs } from "../components/ui";
import { formatDateTime } from "../utils/fileDisplay";
import { applyLocalDecryptionRevealTokens, browserRevealTokenStorage, saveLocalDecryptionRevealToken } from "../utils/decryptionRevealTokens";

export const FILE_CENTER_SCOPE_TABS: Array<{ key: FileCenterScope; label: string; description: string }> = [
  { key: "tenant_cloud", label: "全部密文", description: "查看当前租户内全部密文，下载后由本地密钥决定是否恢复明文。" },
  { key: "owned_by_me", label: "我的加密文件", description: "查看由当前账号创建的密文。" }
];

/** FileCenterPage 只把文件中心作为密文仓库使用，是否能恢复明文完全留给 Electron 本地密钥流程。 */
export function FileCenterPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const initialScope = normalizeScope(searchParams.get("tab"));
  const [scope, setScope] = useState<FileCenterScope>(initialScope);
  const [items, setItems] = useState<EncryptedFileRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState("");
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [ownerFilter, setOwnerFilter] = useState("");
  const [algorithmFilter, setAlgorithmFilter] = useState("");
  const [lastUpdatedAt, setLastUpdatedAt] = useState("");
  const [working, setWorking] = useState("");
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailFile, setDetailFile] = useState<EncryptedFileRecord | null>(null);
  const [detail, setDetail] = useState<Record<string, unknown> | null>(null);
  const hasFilters = Boolean(keyword || statusFilter || ownerFilter || algorithmFilter);

  useEffect(() => { setScope(normalizeScope(searchParams.get("tab"))); }, [searchParams]);

  useEffect(() => {
    setPage(1);
    setKeyword("");
    setStatusFilter("");
    setOwnerFilter("");
    setAlgorithmFilter("");
    setDetailOpen(false);
    setDetail(null);
    setDetailFile(null);
  }, [scope, auth.currentUserId, auth.currentTenantId]);

  useEffect(() => { void load(); }, [scope, page, auth.currentUserId, auth.currentTenantId]);

  const filteredItems = useMemo(() => items.filter((file) => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    const keywordMatched = !normalizedKeyword || file.original_filename.toLowerCase().includes(normalizedKeyword);
    const statusMatched = !statusFilter || file.status === statusFilter;
    return keywordMatched && statusMatched && ownerMatched(file, ownerFilter) && algorithmMatched(file, algorithmFilter);
  }), [items, keyword, statusFilter, ownerFilter, algorithmFilter]);

  async function load() {
    setLoading(true);
    setNotice("");
    try {
      const result = await listFileCenterItems(scope, page, 20);
      setItems(applyLocalDecryptionRevealTokens(result.items, browserRevealTokenStorage(), auth.currentTenantId, auth.currentUserId));
      setTotal(result.total);
      setLastUpdatedAt(new Date().toISOString());
    } catch (error) {
      setItems([]);
      setTotal(0);
      setNotice(error instanceof Error ? error.message : "文件中心加载失败");
    } finally {
      setLoading(false);
    }
  }

  function changeScope(next: string) {
    const nextScope = normalizeScope(next);
    setSearchParams({ tab: nextScope });
    setScope(nextScope);
  }

  function resetFilters() {
    setKeyword("");
    setStatusFilter("");
    setOwnerFilter("");
    setAlgorithmFilter("");
  }

  async function decrypt(file: EncryptedFileRecord) {
    if (String(file.status).toUpperCase() !== "AVAILABLE" && !file.local_decryption_reveal_token) return;
    if (!window.desktopEncryption) {
      message.warning("请在 Electron 桌面端执行本地解密。");
      return;
    }
    setWorking(file.id);
    try {
      if (file.local_decryption_reveal_token) {
        const revealed = await window.desktopEncryption.revealDecryptedFile(file.local_decryption_reveal_token);
        if (revealed) {
          message.success("已打开本地保存位置。");
          return;
        }
        // 打开目录失败不能删除下载标识，否则本地路径暂时不可用时会被误判为未下载。
        message.info("暂时无法打开本地文件夹，已下载标识仍会保留。");
        return;
      }
      const result = await window.desktopEncryption.decryptFile({
        accountId: auth.currentUserId,
        tenantId: auth.currentTenantId,
        apiBaseUrl: new URL(API_BASE_URL).origin,
        accessToken: auth.accessToken,
        fileId: file.id,
        suggestedFilename: file.original_filename
      });
      if (!result.cancelled && result.decryptionFailed) {
        if (result.revealToken) {
          saveLocalDecryptionRevealToken(browserRevealTokenStorage(), auth.currentTenantId, auth.currentUserId, file.id, result.revealToken);
          patchFile(file.id, { local_decryption_reveal_token: result.revealToken });
        }
        const opened = result.revealToken ? await window.desktopEncryption.revealDecryptedFile(result.revealToken) : false;
        message.warning(`密文下载成功，当前本地密钥无法恢复明文；已保存原始密文：${result.preservedCiphertextFilename ?? `${file.original_filename}.enc`}${opened ? "，已打开保存文件夹" : ""}`);
        return;
      }
      if (!result.cancelled) message.success(`下载并解密成功；已在本地恢复明文：${result.filename ?? result.outputFilename ?? file.original_filename}`);
      if (!result.cancelled && result.metrics?.success) {
        saveLocalDecryptionRevealToken(browserRevealTokenStorage(), auth.currentTenantId, auth.currentUserId, file.id, result.revealToken);
        patchFile(file.id, { local_decryption_reveal_token: result.revealToken, decryption_metrics: { ...result.metrics, lastSuccessfulAt: new Date().toISOString(), successfulCount: 1 } });
      } else if (!result.cancelled && result.revealToken) {
        saveLocalDecryptionRevealToken(browserRevealTokenStorage(), auth.currentTenantId, auth.currentUserId, file.id, result.revealToken);
        patchFile(file.id, { local_decryption_reveal_token: result.revealToken });
      }
    } catch (error) {
      const preserved = error && typeof error === "object" && "preservedCiphertextFilename" in error ? String((error as { preservedCiphertextFilename?: string }).preservedCiphertextFilename ?? "") : "";
      message.warning(preserved ? `密文下载成功，当前本地密钥无法恢复明文；已保存原始密文：${preserved}` : (error instanceof Error ? error.message : "密文下载或本地解密失败"));
    } finally {
      setWorking("");
    }
  }

  async function openFolder(file: EncryptedFileRecord) {
    if (!window.desktopEncryption || !file.local_decryption_reveal_token) return;
    const opened = await window.desktopEncryption.revealDecryptedFile(file.local_decryption_reveal_token);
    if (!opened) message.info("暂时无法打开本地文件夹，已下载标识仍会保留。");
  }

  function patchFile(fileId: string, patch: Partial<EncryptedFileRecord>) {
    setItems((current) => current.map((item) => item.id === fileId ? { ...item, ...patch } : item));
    setDetailFile((current) => current?.id === fileId ? { ...current, ...patch } : current);
  }

  async function download(file: EncryptedFileRecord) {
    if (!window.desktopEncryption) {
      message.warning("请在 Electron 桌面端保存密文。");
      return;
    }
    try {
      const result = await window.desktopEncryption.saveCiphertext({ accountId: auth.currentUserId, tenantId: auth.currentTenantId, apiBaseUrl: new URL(API_BASE_URL).origin, accessToken: auth.accessToken, fileId: file.id, suggestedFilename: file.original_filename });
      if (result.revealToken) {
        saveLocalDecryptionRevealToken(browserRevealTokenStorage(), auth.currentTenantId, auth.currentUserId, file.id, result.revealToken);
        patchFile(file.id, { local_decryption_reveal_token: result.revealToken });
      }
      if (!result.cancelled) message.success(`原始密文已保存：${result.outputFilename}`);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "下载密文失败");
    }
  }

  async function openDetail(file: EncryptedFileRecord) {
    setDetailFile(file);
    setDetail(null);
    setDetailOpen(true);
    try { setDetail(await getFileCenterDetail(file.id) as unknown as Record<string, unknown>); } catch { setDetail(null); }
  }

  return (
    <PageShell className="file-center-page">
      <PageHeader
        title="文件中心"
        description="浏览当前租户内的加密文件。服务端始终提供密文，能否恢复明文由本地密钥决定。"
        actions={<>
          {lastUpdatedAt && <span className="page-last-updated">最后更新：{formatDateTime(lastUpdatedAt).slice(0, 16)}</span>}
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>
          {auth.hasPermission("file.upload") && <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate("/encryption")}>加密并上传</Button>}
        </>}
      />
      {notice && <Alert type="error" showIcon closable message={notice} onClose={() => setNotice("")} />}
      <PageTabs activeKey={scope} items={FILE_CENTER_SCOPE_TABS.map((tab) => ({ key: tab.key, label: tab.label }))} onChange={changeScope} />
      <ContentCard className="file-center-card">
        <FilterToolbar resultCount={`共 ${hasFilters ? filteredItems.length : total} 个密文`} actions={hasFilters ? <Button onClick={resetFilters}>重置筛选</Button> : undefined}>
          <Input.Search allowClear placeholder="搜索文件名" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
          {scope !== "owned_by_me" && <Input allowClear placeholder="文件拥有者" value={ownerFilter} onChange={(event) => setOwnerFilter(event.target.value)} />}
          {scope === "owned_by_me" && <Select value={statusFilter} onChange={setStatusFilter} options={[{ value: "", label: "全部状态" }, { value: "AVAILABLE", label: "可用" }, { value: "PENDING", label: "处理中" }, { value: "FAILED", label: "失败" }]} />}
          <Select value={algorithmFilter} onChange={setAlgorithmFilter} options={[{ value: "", label: "全部算法" }, { value: "RSA", label: "RSA-OAEP" }, { value: "BSW07", label: "BSW07" }, { value: "Waters11", label: "Waters11" }, { value: "TKN20", label: "TKN20" }]} />
        </FilterToolbar>
        {loading && items.length === 0 ? <LoadingSkeleton rows={5} /> : <FileTable scope={scope} items={filteredItems} total={hasFilters ? filteredItems.length : total} page={page} loading={loading} workingId={working} onPage={setPage} onDecrypt={(file) => void decrypt(file)} onOpenFolder={(file) => void openFolder(file)} onDownload={(file) => void download(file)} onDetail={(file) => void openDetail(file)} emptySearch={hasFilters} />}
      </ContentCard>
      <EncryptedFileDetail open={detailOpen} detail={detail} fallbackFile={detailFile} decrypting={Boolean(working)} onClose={() => setDetailOpen(false)} onDecrypt={(file) => void decrypt(file)} onDownload={(file) => void download(file)} />
    </PageShell>
  );
}

function ownerMatched(file: EncryptedFileRecord, keyword: string) {
  if (!keyword.trim()) return true;
  const owner = `${file.owner?.display_name ?? ""} ${file.owner?.nickname ?? ""} ${file.owner?.email ?? ""}`.toLowerCase();
  return owner.includes(keyword.trim().toLowerCase());
}

function algorithmMatched(file: EncryptedFileRecord, keyword: string) {
  if (!keyword) return true;
  return `${file.algorithm?.dek_algorithm ?? ""} ${file.algorithm?.algorithm_code ?? ""}`.toLowerCase().includes(keyword.toLowerCase());
}

function normalizeScope(value: string | null): FileCenterScope {
  return value === "owned_by_me" ? value : "tenant_cloud";
}
