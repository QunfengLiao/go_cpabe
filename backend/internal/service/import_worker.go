package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"go-cpabe/backend/internal/domain"
	"gorm.io/gorm"
)

// ImportWorkerConfig 控制后台导入的轮询、租约和单次批量写入规模。
type ImportWorkerConfig struct {
	PollInterval time.Duration
	Lease        time.Duration
	BulkSize     int
}

// ImportWorker 从 MySQL 持久化队列领取批次；租约使多实例部署和进程重启都能安全恢复。
type ImportWorker struct {
	service *ImportService
	config  ImportWorkerConfig
}

// NewImportWorker 创建后台导入执行器，并对异常配置提供保守默认值。
func NewImportWorker(service *ImportService, config ImportWorkerConfig) *ImportWorker {
	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.Lease < 10*time.Second {
		config.Lease = 2 * time.Minute
	}
	if config.BulkSize <= 0 {
		config.BulkSize = 300
	}
	return &ImportWorker{service: service, config: config}
}

// Run 持续领取可用批次；单批失败只记录结果，不会终止后续批次消费。
func (w *ImportWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()
	for {
		if err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("import worker run once: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunOnce 原子领取并执行一个批次，便于测试和受控运维调用。
func (w *ImportWorker) RunOnce(ctx context.Context) error {
	now := time.Now()
	batch, err := w.service.batches.ClaimNextImportBatch(ctx, now, w.config.Lease)
	if err != nil || batch == nil {
		return err
	}
	return w.execute(ctx, batch)
}

// execute 校验服务端快照并在单一业务事务内导入，任何错误都会整体回滚并落地终态。
func (w *ImportWorker) execute(ctx context.Context, batch *domain.TenantImportBatch) error {
	rows, err := decodeAndVerifyImportRows(batch.RowsJSON, batch.SnapshotHash)
	if err != nil || hasInvalidRows(rows) {
		return w.fail(ctx, batch, "批次快照校验失败")
	}
	if err := w.service.batches.UpdateImportProgress(ctx, batch.BatchID, batch.LeaseToken, "WRITING", 0, time.Now(), w.config.Lease); err != nil {
		return err
	}
	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go w.heartbeat(ctx, batch, stopHeartbeat, heartbeatDone)
	importErr := w.service.batches.Transaction(ctx, func(tx *gorm.DB) error {
		switch batch.ImportType {
		case domain.ImportTypeUsers:
			return w.service.importUsersBulkTx(ctx, tx, batch.TenantID, batch.CreatedBy, rows, w.config.BulkSize, func(phase string, processed int) error {
				batch.ProcessedCount = processed
				return w.service.batches.UpdateImportProgress(ctx, batch.BatchID, batch.LeaseToken, phase, processed, time.Now(), w.config.Lease)
			})
		case domain.ImportTypeOrgUnits:
			return w.service.importOrgUnitsTx(ctx, tx, batch.TenantID, batch.CreatedBy, rows)
		default:
			return ErrImportFileInvalid
		}
	})
	close(stopHeartbeat)
	<-heartbeatDone
	if importErr != nil {
		log.Printf("import batch %s rolled back: %v", batch.BatchID, importErr)
		return w.fail(ctx, batch, "正式导入失败，整个批次已回滚")
	}
	success := countAction(rows, domain.ImportRowCreate) + countAction(rows, domain.ImportRowUpdate)
	skipped := countAction(rows, domain.ImportRowSkip)
	if err := w.service.batches.CompleteImportBatch(ctx, batch.BatchID, batch.LeaseToken, success, skipped, time.Now()); err != nil {
		return err
	}
	w.service.recordAudit(ctx, batch.TenantID, batch.CreatedBy, "tenant.import.completed", batch.ImportType, "SUCCESS", map[string]any{"batch_id": batch.BatchID, "success_count": success})
	return nil
}

// decodeAndVerifyImportRows 以 Go 结构重新规范化 JSON 后校验摘要，兼容 MySQL JSON 列对键顺序和空白的重写。
func decodeAndVerifyImportRows(encoded []byte, expectedHash string) ([]domain.ImportRowResult, error) {
	if expectedHash == "" {
		return nil, ErrImportBatchTampered
	}
	var rows []domain.ImportRowResult
	if err := json.Unmarshal(encoded, &rows); err != nil {
		return nil, ErrImportBatchTampered
	}
	canonical, err := json.Marshal(rows)
	if err != nil || sha256Hex(canonical) != expectedHash {
		return nil, ErrImportBatchTampered
	}
	return rows, nil
}

// heartbeat 在长事务期间续租；即使进程中断，租约到期后其他实例仍可重新领取。
func (w *ImportWorker) heartbeat(ctx context.Context, batch *domain.TenantImportBatch, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	interval := w.config.Lease / 3
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case now := <-ticker.C:
			if err := w.service.batches.UpdateImportProgress(ctx, batch.BatchID, batch.LeaseToken, "WRITING", batch.ProcessedCount, now, w.config.Lease); err != nil {
				log.Printf("import batch %s heartbeat: %v", batch.BatchID, err)
				return
			}
		}
	}
}

// fail 以当前租约记录脱敏失败原因，避免把 SQL、密码或租户数据暴露到前端。
func (w *ImportWorker) fail(ctx context.Context, batch *domain.TenantImportBatch, reason string) error {
	err := w.service.batches.FailImportBatch(ctx, batch.BatchID, batch.LeaseToken, reason, batch.TotalCount, time.Now())
	if err == nil {
		w.service.recordAudit(ctx, batch.TenantID, batch.CreatedBy, "tenant.import.completed", batch.ImportType, "FAILURE", map[string]any{"batch_id": batch.BatchID, "failure_count": batch.TotalCount})
	}
	return err
}
