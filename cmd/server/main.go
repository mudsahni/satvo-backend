package main

import (
	"log"

	"satvos/internal/config"
	"satvos/internal/handler"
	"satvos/internal/repository/postgres"
	"satvos/internal/router"
	"satvos/internal/service"
	s3storage "satvos/internal/storage/s3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := postgres.NewDB(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	tenantRepo := postgres.NewTenantRepo(db)
	userRepo := postgres.NewUserRepo(db)
	fileRepo := postgres.NewFileMetaRepo(db)

	// Initialize storage
	s3Client, err := s3storage.NewS3Client(cfg.S3)
	if err != nil {
		log.Fatalf("failed to initialize S3 client: %v", err)
	}

	// Initialize services
	authSvc := service.NewAuthService(userRepo, tenantRepo, cfg.JWT)
	fileSvc := service.NewFileService(fileRepo, s3Client, cfg.S3)
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
		log.Fatalf("server failed: %v", err)
	}
}
