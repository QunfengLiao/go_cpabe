package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go-cpabe/backend/internal/pkg/response"
)

const tokenBucketScript = `
local values = redis.call('HMGET', KEYS[1], 'tokens', 'updated')
local tokens = tonumber(values[1]) or tonumber(ARGV[1])
local updated = tonumber(values[2]) or tonumber(ARGV[3])
local elapsed = math.max(0, tonumber(ARGV[3]) - updated)
tokens = math.min(tonumber(ARGV[1]), tokens + elapsed * tonumber(ARGV[2]))
if tokens < 1 then
  redis.call('HMSET', KEYS[1], 'tokens', tokens, 'updated', ARGV[3])
  redis.call('PEXPIRE', KEYS[1], ARGV[4])
  return 0
end
tokens = tokens - 1
redis.call('HMSET', KEYS[1], 'tokens', tokens, 'updated', ARGV[3])
redis.call('PEXPIRE', KEYS[1], ARGV[4])
return 1`

const concurrencyLeaseScript = `
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[2]) then return 0 end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1`

// EncryptionAdmission 使用 Redis 原子脚本限制创建频率和租户并发；Redis 故障时安全拒绝新任务。
type EncryptionAdmission struct {
	redis         *redis.Client
	maxConcurrent int
	leaseTTL      time.Duration
}

// NewEncryptionAdmission 创建任务准入控制器。
func NewEncryptionAdmission(client *redis.Client, maxConcurrent int, leaseTTL time.Duration) *EncryptionAdmission {
	return &EncryptionAdmission{redis: client, maxConcurrent: maxConcurrent, leaseTTL: leaseTTL}
}

// Acquire 依次消费用户与租户令牌，并领取带过期时间的租户并发租约。
func (a *EncryptionAdmission) Acquire(ctx context.Context, tenantID, userID uint64, leaseID string) error {
	if a == nil || a.redis == nil {
		return response.ErrEncryptionAdmissionUnavailable
	}
	now := time.Now()
	if err := a.consume(ctx, fmt.Sprintf("enc:rate:user:%d:%d", tenantID, userID), 10, 10.0/60.0, now); err != nil {
		return err
	}
	if err := a.consume(ctx, fmt.Sprintf("enc:rate:tenant:%d", tenantID), 100, 100.0/60.0, now); err != nil {
		return err
	}
	key := fmt.Sprintf("enc:leases:tenant:%d", tenantID)
	result, err := a.redis.Eval(ctx, concurrencyLeaseScript, []string{key}, now.UnixMilli(), a.maxConcurrent, now.Add(a.leaseTTL).UnixMilli(), leaseID, a.leaseTTL.Milliseconds()).Int()
	if err != nil {
		return response.ErrEncryptionAdmissionUnavailable
	}
	if result != 1 {
		return response.ErrEncryptionConcurrencyLimited
	}
	return nil
}

// Renew 延长当前租约；租约不存在时拒绝续约，避免进程恢复后凭空占用并发配额。
func (a *EncryptionAdmission) Renew(ctx context.Context, tenantID uint64, leaseID string) error {
	if a == nil || a.redis == nil {
		return response.ErrEncryptionAdmissionUnavailable
	}
	key := fmt.Sprintf("enc:leases:tenant:%d", tenantID)
	exists, err := a.redis.ZScore(ctx, key, leaseID).Result()
	if err != nil || exists <= float64(time.Now().UnixMilli()) {
		return response.ErrEncryptionAdmissionUnavailable
	}
	if err := a.redis.ZAdd(ctx, key, redis.Z{Score: float64(time.Now().Add(a.leaseTTL).UnixMilli()), Member: leaseID}).Err(); err != nil {
		return response.ErrEncryptionAdmissionUnavailable
	}
	return nil
}

// Release 在所有终态路径释放租约；重复释放是幂等操作。
func (a *EncryptionAdmission) Release(ctx context.Context, tenantID uint64, leaseID string) {
	if a == nil || a.redis == nil {
		return
	}
	_ = a.redis.ZRem(ctx, fmt.Sprintf("enc:leases:tenant:%d", tenantID), leaseID).Err()
}

// SweepExpired 删除租户已过期租约，供后台巡检和诊断复用。
func (a *EncryptionAdmission) SweepExpired(ctx context.Context, tenantID uint64) error {
	if a == nil || a.redis == nil {
		return response.ErrEncryptionAdmissionUnavailable
	}
	return a.redis.ZRemRangeByScore(ctx, fmt.Sprintf("enc:leases:tenant:%d", tenantID), "-inf", fmt.Sprintf("%d", time.Now().UnixMilli())).Err()
}

// consume 通过 Lua 原子补充和消费一个令牌桶。
func (a *EncryptionAdmission) consume(ctx context.Context, key string, capacity int, refillPerSecond float64, now time.Time) error {
	result, err := a.redis.Eval(ctx, tokenBucketScript, []string{key}, capacity, refillPerSecond/1000, now.UnixMilli(), int64((time.Hour).Milliseconds())).Int()
	if err != nil {
		return response.ErrEncryptionAdmissionUnavailable
	}
	if result != 1 {
		return response.ErrEncryptionRateLimited
	}
	return nil
}
