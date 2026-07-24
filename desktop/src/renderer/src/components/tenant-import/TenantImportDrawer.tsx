import { useEffect, useMemo, useRef, useState } from "react";
import { Alert, Button, Card, Drawer, Empty, Progress, Select, Space, Steps, Table, Tag, Typography, Upload, message } from "antd";
import { CloseOutlined, DownloadOutlined, FileExcelOutlined, InboxOutlined, ReloadOutlined, RightOutlined } from "@ant-design/icons";
import { confirmImport, downloadImportErrors, downloadImportTemplate, getImportBatchStatus, type ImportBatchStatusView, type ImportPreview, type ImportRow, type ImportType, validateImport } from "../../api/import";

const { Dragger } = Upload;

interface TenantImportDrawerProps {
  open: boolean;
  type: ImportType;
  onClose: () => void;
  onCompleted: () => Promise<void> | void;
}

// TenantImportDrawer 提供模板、上传、预览、确认和结果四步流程，所有校验结果以服务端返回为准。
export function TenantImportDrawer({ open, type, onClose, onCompleted }: TenantImportDrawerProps) {
  const [step, setStep] = useState(0);
  const [file, setFile] = useState<File>();
  const [preview, setPreview] = useState<ImportPreview>();
  const [loading, setLoading] = useState(false);
  const [validationError, setValidationError] = useState<string>();
  const [confirmationError, setConfirmationError] = useState<string>();
  const [batchStatus, setBatchStatus] = useState<ImportBatchStatusView>();
  const [onlyErrors, setOnlyErrors] = useState(false);
  const [actionFilter, setActionFilter] = useState<"ALL" | ImportRow["action"]>("ALL");
  const completedBatchRef = useRef<string | undefined>(undefined);
  const isProcessing = preview?.status === "QUEUED" || preview?.status === "IMPORTING";

  useEffect(() => {
    if (!open || preview) return;
    const batchId = readPendingBatch(type);
    if (!batchId) return;
    let disposed = false;
    void getImportBatchStatus(batchId).then((status) => {
      if (disposed) return;
      setBatchStatus(status);
      setPreview({ batch_id: batchId, status: status.status, limits: { max_file_size: 0, max_rows: 0, batch_ttl_seconds: 0 }, summary: { total: status.total, valid: status.total, created: 0, updated: 0, skipped: status.skipped, failed: status.failed }, rows: [] });
      setStep(3);
    }).catch(() => clearPendingBatch(type));
    return () => { disposed = true; };
  }, [open, preview, type]);

  useEffect(() => {
    const batchId = preview?.batch_id;
    if (!batchId || (preview.status !== "QUEUED" && preview.status !== "IMPORTING")) return;
    let disposed = false;
    const poll = async () => {
      try {
        const status = await getImportBatchStatus(batchId);
        if (disposed) return;
        setBatchStatus(status);
        setPreview((current) => current?.batch_id === batchId ? { ...current, status: status.status } : current);
        setConfirmationError(undefined);
        if (status.status === "SUCCEEDED" && completedBatchRef.current !== batchId) {
          completedBatchRef.current = batchId;
          clearPendingBatch(type);
          await onCompleted();
        }
        if (status.status === "FAILED" || status.status === "EXPIRED") clearPendingBatch(type);
      } catch (error) {
        if (!disposed) setConfirmationError(error instanceof Error ? error.message : "查询导入进度失败，系统将自动重试");
      }
    };
    void poll();
    const timer = window.setInterval(() => void poll(), 1500);
    return () => { disposed = true; window.clearInterval(timer); };
  }, [onCompleted, preview?.batch_id, preview?.status, type]);

  const rows = useMemo(() => (preview?.rows ?? []).filter((row) => {
    if (onlyErrors && row.status !== "INVALID") return false;
    return actionFilter === "ALL" || row.action === actionFilter;
  }), [actionFilter, onlyErrors, preview?.rows]);

  async function downloadTemplate() {
    setLoading(true);
    try {
      saveBlob(await downloadImportTemplate(type), type === "users" ? "租户用户导入模板.xlsx" : "组织架构导入模板.xlsx");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "模板下载失败");
    } finally {
      setLoading(false);
    }
  }

  async function validate() {
    if (!file) return;
    setValidationError(undefined);
    setLoading(true);
    try {
      const nextPreview = await validateImport(type, file);
      setPreview(nextPreview);
      setStep(2);
    } catch (error) {
      const detail = error instanceof Error ? error.message : "文件校验失败";
      setValidationError(detail);
      message.error(detail);
    } finally {
      setLoading(false);
    }
  }

  async function confirm() {
    if (!preview || preview.summary.failed > 0 || loading) return;
    setLoading(true);
    setConfirmationError(undefined);
    try {
      const result = await confirmImport(type, preview.batch_id);
      writePendingBatch(type, result.batch_id);
      // 确认接口为保证快速受理不再返回万行快照，保留当前预览统计和行数据，只更新后台状态。
      setPreview((current) => current ? { ...current, status: result.status } : result);
      setBatchStatus({ batch_id: result.batch_id, status: result.status, phase: "WAITING", total: result.summary.total, processed: 0, success: 0, failed: 0, skipped: result.summary.skipped, attempt_count: 0 });
      setStep(3);
    } catch (error) {
      const detail = error instanceof Error ? error.message : "导入任务提交失败";
      setConfirmationError(detail);
      message.error(detail);
    } finally {
      setLoading(false);
    }
  }

  async function downloadErrors() {
    if (!preview) return;
    try {
      saveBlob(await downloadImportErrors(preview.batch_id), `导入错误报告-${preview.batch_id}.xlsx`);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "错误报告下载失败");
    }
  }

  function resetFile() {
    if (isProcessing) return;
    setFile(undefined);
    setPreview(undefined);
    setValidationError(undefined);
    setConfirmationError(undefined);
    setBatchStatus(undefined);
    completedBatchRef.current = undefined;
    clearPendingBatch(type);
    setStep(1);
  }

  return (
    <Drawer
      open={open}
      className="tenant-import-drawer"
      width="min(960px, 100vw)"
      title={type === "users" ? "批量导入租户用户" : "批量导入组织架构"}
      onClose={() => !loading && onClose()}
      extra={<Button icon={<ReloadOutlined />} onClick={resetFile} disabled={loading || isProcessing}>重新开始</Button>}
      destroyOnClose
    >
      <div className="tenant-import-shell">
        <Steps className="tenant-import-steps" current={step} items={[{ title: "下载模板" }, { title: "上传文件" }, { title: "数据预览" }, { title: "导入结果" }]} />
        <div className="tenant-import-content">

      {step === 0 && (
        <section className="tenant-import-step">
          <Typography.Title level={4}>先下载对应模板</Typography.Title>
          <Typography.Paragraph type="secondary">模板内置隐藏的系统映射行，请勿取消隐藏或修改；请从中文表头下方填写数据，字段说明与数据字典工作表提供示例和可选值。只支持后端限制范围内的 .xlsx 文件。</Typography.Paragraph>
          <div className="tenant-import-actions">
            <Button type="primary" icon={<DownloadOutlined />} loading={loading} onClick={() => void downloadTemplate()}>下载模板</Button>
            <Button onClick={() => setStep(1)}>下一步</Button>
          </div>
        </section>
      )}

      {step === 1 && (
        <section className="tenant-import-step">
          <Typography.Title level={4}>上传并开始校验</Typography.Title>
          <Alert type="info" showIcon message="文件上传只生成预校验批次，不会立即修改用户、组织、角色或属性。文件大小和最大行数以服务端响应为准。" />
          <div className="tenant-import-upload">
            <Dragger
              accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              maxCount={1}
              multiple={false}
              disabled={loading}
              showUploadList={false}
              beforeUpload={(next) => {
                setFile(next);
                setPreview(undefined);
                setValidationError(undefined);
                return false;
              }}
            >
              <p className="ant-upload-drag-icon"><InboxOutlined /></p>
              <p className="ant-upload-text">拖拽 .xlsx 文件到这里，或点击选择文件</p>
              <p className="ant-upload-hint">不要删除或修改必要字段名称；用户导入前请确保组织编码已存在。</p>
            </Dragger>
          </div>
          {file && (
            <div className="tenant-import-selected-file">
              <FileExcelOutlined aria-hidden />
              <Typography.Text className="tenant-import-selected-file-name" ellipsis={{ tooltip: file.name }}>{file.name}</Typography.Text>
              <Button type="text" size="small" icon={<CloseOutlined />} disabled={loading} aria-label="移除已选文件" onClick={() => { setFile(undefined); setValidationError(undefined); }} />
            </div>
          )}
          {loading && <Alert type="info" showIcon message="正在上传并校验文件" description="大批量用户需要计算密码摘要，可能耗时较长；完成后会自动进入“数据预览”，请勿关闭窗口。" />}
          {validationError && <Alert type="error" showIcon message="校验未通过" description={validationError} closable onClose={() => setValidationError(undefined)} />}
          <Typography.Text type="secondary">校验成功后将自动进入下一步“数据预览”。</Typography.Text>
          <div className="tenant-import-actions">
            <Button onClick={() => setStep(0)} disabled={loading}>上一步</Button>
            <Button type="primary" icon={<RightOutlined />} disabled={!file} loading={loading} onClick={() => void validate()}>校验并进入预览</Button>
          </div>
        </section>
      )}

      {step === 2 && preview && (
        <section className="tenant-import-step">
          <Typography.Title level={4}>服务端预览与校验</Typography.Title>
          <div className="tenant-import-summary-grid">
            <Card size="small"><Typography.Text type="secondary">总行数</Typography.Text><Typography.Title level={3}>{preview.summary.total}</Typography.Title></Card>
            <Card size="small"><Typography.Text type="secondary">新增</Typography.Text><Typography.Title level={3}>{preview.summary.created}</Typography.Title></Card>
            <Card size="small"><Typography.Text type="secondary">更新</Typography.Text><Typography.Title level={3}>{preview.summary.updated}</Typography.Title></Card>
            <Card size="small"><Typography.Text type="secondary">错误</Typography.Text><Typography.Title level={3} type={preview.summary.failed ? "danger" : undefined}>{preview.summary.failed}</Typography.Title></Card>
          </div>
          {preview.summary.failed > 0 && <Alert type="error" showIcon message="存在校验错误，修复文件后重新上传；错误批次不能确认导入。" action={<Button size="small" onClick={() => void downloadErrors()}>下载错误报告</Button>} />}
          <Space wrap>
            <Button type={onlyErrors ? "primary" : "default"} onClick={() => setOnlyErrors((value) => !value)}>仅看错误</Button>
            <Select value={actionFilter} onChange={setActionFilter} options={[{ value: "ALL", label: "全部动作" }, { value: "CREATE", label: "待新增" }, { value: "UPDATE", label: "待更新" }, { value: "SKIP", label: "将跳过" }]} />
          </Space>
          <Table<ImportRow>
            rowKey="row_number"
            size="small"
            pagination={{ pageSize: 20, showSizeChanger: false }}
            dataSource={rows}
            scroll={{ x: "max-content" }}
            columns={[
              { title: "Excel 行号", dataIndex: "row_number", width: 100 },
              { title: "关键字段", dataIndex: "key", ellipsis: true },
              { title: "处理动作", dataIndex: "action", render: (value: ImportRow["action"]) => <Tag color={value === "CREATE" ? "green" : value === "UPDATE" ? "blue" : "default"}>{value === "CREATE" ? "待新增" : value === "UPDATE" ? "待更新" : "将跳过"}</Tag> },
              { title: "校验状态", dataIndex: "status", render: (value: ImportRow["status"]) => value === "VALID" ? <Tag color="success">通过</Tag> : <Tag color="error">失败</Tag> },
              { title: "错误信息", dataIndex: "errors", render: (errors: ImportRow["errors"]) => <Typography.Text type="danger" ellipsis={{ tooltip: errors?.map((item) => `${item.field}: ${item.message}`).join("；") }}>{errors?.map((item) => `${item.field}: ${item.message}`).join("；") || "-"}</Typography.Text> }
            ]}
          />
          <div className="tenant-import-actions"><Button onClick={() => setStep(1)}>返回重新上传</Button><Button type="primary" disabled={preview.summary.failed > 0} loading={loading} onClick={() => void confirm()}>确认导入</Button></div>
        </section>
      )}

      {step === 3 && preview && (
        <section className="tenant-import-step">
          {(preview.status === "QUEUED" || preview.status === "IMPORTING") && <>
            <Alert type="info" showIcon message={preview.status === "QUEUED" ? "导入任务已排队" : "正在后台导入"} description="可以关闭窗口，重新打开后仍可继续查看该批次；请勿重复提交同一批次。" />
            <Progress percent={Math.min(99, Math.round(((batchStatus?.processed ?? 0) / Math.max(batchStatus?.total ?? preview.summary.total, 1)) * 100))} status="active" format={(percent) => `${batchStatus?.phase === "WRITING" ? "批量写入" : "准备中"} ${percent}%`} />
            <Typography.Text type="secondary">已处理 {batchStatus?.processed ?? 0} / {batchStatus?.total ?? preview.summary.total}，执行次数 {batchStatus?.attempt_count ?? 0}</Typography.Text>
          </>}
          {preview.status === "SUCCEEDED" && <Alert type="success" showIcon message="导入成功" description={`批次 ${preview.batch_id} 已完成，共写入 ${batchStatus?.success ?? (preview.summary.created + preview.summary.updated)} 条。`} />}
          {preview.status === "FAILED" && <Alert type="error" showIcon message="导入失败，批次已回滚" description={batchStatus?.failure_reason || `批次 ${preview.batch_id} 未留下部分写入，请下载错误报告或重新上传。`} />}
          {confirmationError && <Alert type="warning" showIcon message="暂时无法获取导入进度" description={confirmationError} />}
          <div className="tenant-import-actions">{preview.status === "FAILED" && <Button onClick={() => void downloadErrors()} icon={<DownloadOutlined />}>下载错误报告</Button>}<Button type="primary" onClick={onClose}>返回管理页面</Button></div>
        </section>
      )}
      {!preview && step > 1 && <Empty description="暂无预览数据" />}
        </div>
      </div>
    </Drawer>
  );
}

function saveBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

function pendingBatchKey(type: ImportType): string {
  return `tenant-import-pending:${type}`;
}

function readPendingBatch(type: ImportType): string | undefined {
  try {
    return window.localStorage.getItem(pendingBatchKey(type)) || undefined;
  } catch {
    return undefined;
  }
}

function writePendingBatch(type: ImportType, batchId: string): void {
  try {
    window.localStorage.setItem(pendingBatchKey(type), batchId);
  } catch {
    // 本地恢复能力不可用时仍可依赖服务端批次历史，不阻断导入主链路。
  }
}

function clearPendingBatch(type: ImportType): void {
  try {
    window.localStorage.removeItem(pendingBatchKey(type));
  } catch {
    // 浏览器禁用存储时无需额外处理。
  }
}
