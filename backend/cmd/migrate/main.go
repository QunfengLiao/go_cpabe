package main

import (
	"context"
	"log"
	"time"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/migrations"
)

// main 执行数据库结构迁移；HTTP server 默认不再承担 information_schema 检查。
func main() {
	startedAt := time.Now()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := config.OpenDatabase(cfg)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	if err := migrations.RunExplicitMigrations(db); err != nil {
		log.Fatalf("显式 SQL 迁移失败: %v", err)
	}
	if err := migrations.AutoMigrate(db); err != nil {
		log.Fatalf("Gorm 模型同步失败: %v", err)
	}
	if err := migrations.ValidateRBACMigration(context.Background(), db); err != nil {
		log.Fatalf("RBAC 迁移验证失败: %v", err)
	}
	log.Printf("数据库迁移完成，耗时 %s", time.Since(startedAt).Round(time.Millisecond))
}
