package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"satvos/internal/config"
	"satvos/internal/handler"
	"satvos/internal/parser"
	claudeparser "satvos/internal/parser/claude"
	geminiparser "satvos/internal/parser/gemini"
	"satvos/internal/port"
	"satvos/internal/repository/postgres"
	"satvos/internal/router"
	"satvos/internal/service"
	s3storage "satvos/internal/storage/s3"
	"satvos/internal/validator"
	"satvos/internal/validator/invoice"

	_ "satvos/docs" // swagger docs
)

// @title SATVOS API
// @version 1.0
// @description Multi-tenant document processing service with AI-powered invoice parsing and GST validation.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@satvos.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the JWT token. Example: Bearer eyJhbGci...

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
	documentTagRepo := postgres.NewDocumentTagRepo(db)
	validationRuleRepo := postgres.NewDocumentValidationRuleRepo(db)

	// Register parser providers
	parser.RegisterProvider("claude", func(provCfg *config.ParserProviderConfig) (port.DocumentParser, error) {
		return claudeparser.NewParser(provCfg), nil
	})
	parser.RegisterProvider("gemini", func(provCfg *config.ParserProviderConfig) (port.DocumentParser, error) {
		return geminiparser.NewParser(provCfg), nil
	})

	// Initialize primary parser
	primaryCfg := cfg.Parser.PrimaryConfig()
	documentParser, err := parser.NewParser(primaryCfg)
	if err != nil {
		return fmt.Errorf("failed to create primary parser: %w", err)
	}

	// Initialize optional merge parser for dual-parse mode
	var mergeDocParser port.DocumentParser
	if secondaryCfg := cfg.Parser.SecondaryConfig(); secondaryCfg != nil {
		secondaryParser, secErr := parser.NewParser(secondaryCfg)
		if secErr != nil {
			log.Printf("WARNING: failed to create secondary parser (%v), dual-parse mode will be unavailable", secErr)
		} else {
			mergeDocParser = parser.NewMergeParser(documentParser, secondaryParser)
			log.Printf("Multi-parser mode enabled: primary=%s, secondary=%s", primaryCfg.Provider, secondaryCfg.Provider)
		}
	}

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
	var documentSvc service.DocumentService
	if mergeDocParser != nil {
		documentSvc = service.NewDocumentServiceWithMerge(docRepo, fileRepo, collectionPermRepo, documentTagRepo, documentParser, mergeDocParser, s3Client, validationEngine)
	} else {
		documentSvc = service.NewDocumentService(docRepo, fileRepo, collectionPermRepo, documentTagRepo, documentParser, s3Client, validationEngine)
	}

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
