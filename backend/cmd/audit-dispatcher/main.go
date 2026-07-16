package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// main 单次领取并投递有限批审计 outbox；外部调度器可重复运行，多实例依靠数据库租约互斥。
func main() {
	limit := flag.Int("limit", 0, "单次最多处理事件数；0 使用环境配置")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	batchSize := cfg.AuditDispatchBatchSize
	if *limit > 0 {
		batchSize = *limit
	}
	db, err := config.OpenDatabase(cfg)
	if err != nil {
		log.Fatal("连接数据库失败: AUDIT_DATABASE_UNAVAILABLE")
	}
	repo := repository.NewGormAuditOutboxRepository(db)
	dispatcher, err := service.NewAuditDispatcherService(repo, service.AuditDispatcherConfig{BatchSize: batchSize, Lease: cfg.AuditDispatchLease, MaxRetries: uint32(cfg.AuditDispatchMaxRetries), BaseBackoff: cfg.AuditDispatchBaseBackoff, MaxBackoff: cfg.AuditDispatchMaxBackoff, DeliveredRetention: cfg.AuditDeliveredRetention})
	if err != nil {
		log.Fatal("初始化审计 Dispatcher 失败: AUDIT_DISPATCH_CONFIG_INVALID")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	result, err := dispatcher.RunOnce(ctx)
	if err != nil {
		log.Fatal("审计投递失败: AUDIT_DISPATCH_RUN_FAILED")
	}
	log.Printf("审计投递完成: 领取=%d 投递=%d 重试=%d 死信=%d 租约失效=%d 清理=%d", result.Claimed, result.Delivered, result.Retried, result.Dead, result.LeaseLost, result.Deleted)
}
