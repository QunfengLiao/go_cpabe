package main

import (
	"fmt"
	"log"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	mysqlDB, mysqlErr := repository.OpenMySQL(cfg.MySQL)
	if mysqlErr != nil {
		log.Printf("mysql init warning: %v", mysqlErr)
	}

	redisClient, redisErr := repository.OpenRedis(cfg.Redis)
	if redisErr != nil {
		log.Printf("redis init warning: %v", redisErr)
	}

	r := router.New(router.Dependencies{
		Config:     cfg,
		MySQL:      mysqlDB,
		MySQLError: mysqlErr,
		Redis:      redisClient,
		RedisError: redisErr,
	})

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	log.Printf("server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
