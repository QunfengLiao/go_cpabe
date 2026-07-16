package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// main 装配可重复运行的孤儿对象与过期暂存清理命令，并响应进程终止信号。
func main() {
	limit := flag.Int("limit", 100, "单次最多处理对象数")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := config.OpenDatabase(cfg)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	repositoryLayer := repository.NewGormEncryptionRepository(db)
	audit := service.NewDatabaseAuditRecorder(repository.NewGormAuditRepository(db))
	storageLayer := storage.NewLocalEncryptedFileStorage(cfg.EncryptedFileStorageDir, cfg.EncryptedFileTempDir)
	cleanup := service.NewOrphanCleanupService(repositoryLayer, storageLayer, audit, cfg.EncryptionStagingTTL)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	result, err := cleanup.Run(ctx, *limit)
	if err != nil {
		log.Fatalf("清理失败: %v", err)
	}
	log.Printf("清理完成: 领取=%d 成功=%d 失败=%d 过期暂存=%d", result.Claimed, result.Cleaned, result.Failed, result.ExpiredStaging)
}
