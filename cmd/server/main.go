package main

import (
	"fmt"
	"log"

	"satvos/internal/config"
	"satvos/internal/handler"
	"satvos/internal/repository/postgres"
	"satvos/internal/router"
	"satvos/internal/service"
	s3storage "satvos/internal/storage/s3"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	db, err := postgres.NewDB(&cfg.DB)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Initialize repositories
	tenantRepo := postgres.NewTenantRepo(db)
	userRepo := postgres.NewUserRepo(db)
	fileRepo := postgres.NewFileMetaRepo(db)

	// Initialize storage
	s3Client, err := s3storage.NewS3Client(&cfg.S3)
	if err != nil {
		return fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Initialize services
	authSvc := service.NewAuthService(userRepo, tenantRepo, cfg.JWT)
	fileSvc := service.NewFileService(fileRepo, s3Client, &cfg.S3)
	tenantSvc := service.NewTenantService(tenantRepo)
	userSvc := service.NewUserService(userRepo)

	// Initialize handlers
	authH := handler.NewAuthHandler(authSvc)
	fileH := handler.NewFileHandler(fileSvc)
	tenantH := handler.NewTenantHandler(tenantSvc)
	userH := handler.NewUserHandler(userSvc)
	healthH := handler.NewHealthHandler(db)

	// Setup router
	r := router.Setup(authSvc, authH, fileH, tenantH, userH, healthH)

	log.Printf("Server starting on %s", cfg.Server.Port)
	if err := r.Run(cfg.Server.Port); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}
