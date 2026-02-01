package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"satvos/internal/config"
	"satvos/internal/handler"
	claudeparser "satvos/internal/parser/claude"
	"satvos/internal/repository/postgres"
	"satvos/internal/router"
	"satvos/internal/service"
	s3storage "satvos/internal/storage/s3"
	"satvos/internal/validator"
	"satvos/internal/validator/invoice"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.SetOutput(os.Stdout)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := postgres.NewDB(&cfg.DB)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize repositories
	tenantRepo := postgres.NewTenantRepo(db)
	userRepo := postgres.NewUserRepo(db)
	fileRepo := postgres.NewFileMetaRepo(db)
	collectionRepo := postgres.NewCollectionRepo(db)
	collectionPermRepo := postgres.NewCollectionPermissionRepo(db)
	collectionFileRepo := postgres.NewCollectionFileRepo(db)

	// Initialize storage
	s3Client, err := s3storage.NewS3Client(&cfg.S3)
	if err != nil {
		return fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Initialize document repositories
	docRepo := postgres.NewDocumentRepo(db)
	validationRuleRepo := postgres.NewDocumentValidationRuleRepo(db)

	// Initialize parser
	documentParser := claudeparser.NewParser(&cfg.Parser)

	// Initialize validation engine
	registry := validator.NewRegistry()
	for _, v := range invoice.AllBuiltinValidators() {
		registry.Register(v)
	}
	validationEngine := validator.NewEngine(registry, validationRuleRepo, docRepo)

	// Initialize services
	authSvc := service.NewAuthService(userRepo, tenantRepo, cfg.JWT)
	fileSvc := service.NewFileService(fileRepo, s3Client, &cfg.S3)
	tenantSvc := service.NewTenantService(tenantRepo)
	userSvc := service.NewUserService(userRepo)
	collectionSvc := service.NewCollectionService(collectionRepo, collectionPermRepo, collectionFileRepo, fileSvc)
	documentSvc := service.NewDocumentService(docRepo, fileRepo, collectionPermRepo, documentParser, s3Client, validationEngine)

	// Initialize handlers
	authH := handler.NewAuthHandler(authSvc)
	fileH := handler.NewFileHandler(fileSvc, collectionSvc)
	tenantH := handler.NewTenantHandler(tenantSvc)
	userH := handler.NewUserHandler(userSvc)
	healthH := handler.NewHealthHandler(db)
	collectionH := handler.NewCollectionHandler(collectionSvc)
	documentH := handler.NewDocumentHandler(documentSvc)

	// Setup router
	r := router.Setup(authSvc, authH, fileH, tenantH, userH, healthH, collectionH, documentH)

	log.Printf("Server starting on %s", cfg.Server.Port)
	if err := r.Run(cfg.Server.Port); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}
