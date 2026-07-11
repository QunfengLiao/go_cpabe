package main

import (
	"context"
	"flag"
	"log"
	"time"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// main 执行显式初始化数据写入；它替代 server 启动路径中的隐式 seed。
func main() {
	includeDemo := flag.Bool("demo", false, "同时写入演示策略、组织和属性数据")
	flag.Parse()

	startedAt := time.Now()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := config.OpenDatabase(cfg)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	userRepo := repository.NewGormUserRepository(db)
	tenantRepo := repository.NewGormTenantRepository(db)
	policyRepo := repository.NewGormPolicyRepository(db)
	orgAttributeRepo := repository.NewGormOrgAttributeRepository(db)
	tenantSvc := service.NewTenantService(tenantRepo, userRepo)
	policySvc := service.NewPolicyService(policyRepo, tenantRepo)
	orgAttributeSvc := service.NewOrgAttributeService(orgAttributeRepo, tenantRepo)
	policySvc.SetOrgAttributeRepository(orgAttributeRepo)

	ctx := context.Background()
	if err := tenantSvc.BootstrapDefaultTenant(ctx); err != nil {
		log.Fatalf("初始化默认租户和基础角色失败: %v", err)
	}
	if *includeDemo {
		if err := policySvc.BootstrapDemoPolicyCatalog(ctx); err != nil {
			log.Fatalf("初始化演示策略失败: %v", err)
		}
		if err := orgAttributeSvc.BootstrapDemoOrgAttributes(ctx); err != nil {
			log.Fatalf("初始化演示组织属性失败: %v", err)
		}
	}
	log.Printf("初始化数据完成，demo=%t，耗时 %s", *includeDemo, time.Since(startedAt).Round(time.Millisecond))
}
