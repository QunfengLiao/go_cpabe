package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/repository"
)

// AuditDispatcherConfig 控制单次审计投递的批次、租约、退避、死信和已投递保留期。
type AuditDispatcherConfig struct {
	// BatchSize 限制单次事务领取数量，避免无界占用数据库连接和行锁。
	BatchSize int
	// Lease 是 PROCESSING 事件允许 Worker 独占处理的最长时间。
	Lease time.Duration
	// MaxRetries 是转入死信前允许的失败次数。
	MaxRetries uint32
	// BaseBackoff 是首次失败后的退避基数。
	BaseBackoff time.Duration
	// MaxBackoff 限制指数退避的最长等待时间。
	MaxBackoff time.Duration
	// DeliveredRetention 是已投递 outbox 行的内部保留期，不影响正式审计日志。
	DeliveredRetention time.Duration
}

// AuditDispatchResult 汇总一次 Dispatcher 运行结果，不包含事件正文或内部错误文本。
type AuditDispatchResult struct {
	// Claimed 是本批取得有效租约的事件数。
	Claimed int
	// Delivered 是成功收敛为正式日志的事件数。
	Delivered int
	// Retried 是本批进入退避状态的事件数。
	Retried int
	// Dead 是本批达到最大重试次数并进入死信的事件数。
	Dead int
	// LeaseLost 是因令牌过期或被接管而未更新的事件数。
	LeaseLost int
	// Deleted 是按保留期清理的已投递 outbox 行数。
	Deleted int64
}

// AuditDispatcherService 通过至少一次领取和 UUID 幂等投递，把 outbox 事件最终收敛为一条正式审计日志。
type AuditDispatcherService struct {
	repository repository.AuditOutboxRepository
	config     AuditDispatcherConfig
	now        func() time.Time
}

// NewAuditDispatcherService 创建审计 Dispatcher，并拒绝会导致无界批次、零租约或无退避的危险配置。
func NewAuditDispatcherService(repository repository.AuditOutboxRepository, config AuditDispatcherConfig) (*AuditDispatcherService, error) {
	if repository == nil || config.BatchSize <= 0 || config.Lease <= 0 || config.MaxRetries == 0 || config.BaseBackoff <= 0 || config.MaxBackoff < config.BaseBackoff || config.DeliveredRetention <= 0 {
		return nil, errors.New("invalid audit dispatcher configuration")
	}
	return &AuditDispatcherService{repository: repository, config: config, now: time.Now}, nil
}

// RunOnce 领取并逐条投递到期事件；单条失败进入退避或死信，不阻断同批其他事件。
func (s *AuditDispatcherService) RunOnce(ctx context.Context) (AuditDispatchResult, error) {
	now := s.now()
	items, err := s.repository.Claim(ctx, s.config.BatchSize, now, s.config.Lease)
	if err != nil {
		return AuditDispatchResult{}, err
	}
	result := AuditDispatchResult{Claimed: len(items)}
	for _, event := range items {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if event.LockToken == nil || *event.LockToken == "" {
			result.LeaseLost++
			continue
		}
		deliveredAt := s.now()
		if err := s.repository.Deliver(ctx, event, *event.LockToken, deliveredAt); err == nil {
			result.Delivered++
			continue
		} else if errors.Is(err, repository.ErrAuditOutboxLeaseLost) {
			result.LeaseLost++
			continue
		}
		nextRetry := deliveredAt.Add(s.retryDelay(event))
		if err := s.repository.MarkFailure(ctx, event, *event.LockToken, "AUDIT_DELIVERY_FAILED", nextRetry, s.config.MaxRetries); err != nil {
			if errors.Is(err, repository.ErrAuditOutboxLeaseLost) {
				result.LeaseLost++
				continue
			}
			return result, err
		}
		if event.RetryCount+1 >= s.config.MaxRetries {
			result.Dead++
		} else {
			result.Retried++
		}
	}
	deleted, err := s.repository.DeleteDeliveredBefore(ctx, now.Add(-s.config.DeliveredRetention), s.config.BatchSize)
	result.Deleted = deleted
	return result, err
}

// retryDelay 计算带稳定小抖动的指数退避；同一事件重试可预测，测试和多实例不会依赖全局随机源。
func (s *AuditDispatcherService) retryDelay(event domain.AuditOutboxEvent) time.Duration {
	exponent := event.RetryCount
	if exponent > 20 {
		exponent = 20
	}
	multiplier := time.Duration(1 << exponent)
	delay := s.config.MaxBackoff
	if s.config.BaseBackoff <= s.config.MaxBackoff/multiplier {
		delay = s.config.BaseBackoff * multiplier
	}
	digest := sha256.Sum256([]byte(event.EventPublicID))
	jitterBasis := binary.BigEndian.Uint16(digest[:2])
	jitter := time.Duration(float64(delay) * (float64(jitterBasis%2000) / 10000.0))
	if delay > time.Duration(math.MaxInt64)-jitter || delay+jitter > s.config.MaxBackoff {
		return s.config.MaxBackoff
	}
	return delay + jitter
}
