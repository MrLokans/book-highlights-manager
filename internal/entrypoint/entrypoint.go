package entrypoint

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/exporters"
	http_controllers "github.com/mrlokans/assistant/internal/http"
)

func Serve(router *gin.Engine, cfg *config.Config) {

	if cfg.Readwise.Token == "" {
		log.Printf("WARNING: Readwise token is not set. Readwise import endpoint will be disabled. Set 'READWISE_TOKEN' environment variable to enable.")
	}

	log.Printf("Checking vault directory: %s\n", cfg.Obsidian.VaultDir)

	if not := cfg.Obsidian.VaultDir == ""; not {
		log.Fatalf("Vault directory is not set")
		return
	}

	// Check export dir exists as is a directory
	if _, err := os.Stat(cfg.Obsidian.VaultDir); os.IsNotExist(err) {
		log.Fatalf("Vault directory %s does not exist", cfg.Obsidian.VaultDir)
		return
	} else {
		log.Printf("Vault directory %s exists\n", cfg.Obsidian.VaultDir)
	}

	// Check export dir is writable by touching and removing an empty file
	_, err := os.Create(fmt.Sprintf("%s/.assistant", cfg.Obsidian.VaultDir))

	// Defer the removal of the temp file file
	defer func() {
		err := os.Remove(fmt.Sprintf("%s/.assistant", cfg.Obsidian.VaultDir))
		if err != nil {
			log.Fatalf("Could not remove the test file from the vault directory %s", cfg.Obsidian.VaultDir)
			return
		}
	}()

	if err != nil {
		log.Fatalf("Vault directory %s is not writable", cfg.Obsidian.VaultDir)
		return
	}

	var timeout = time.Duration(cfg.Global.ShutdownTimeoutInSeconds)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler: router,
	}

	go func() {
		fmt.Printf("Starting server at %s:%d\n", cfg.HTTP.Host, cfg.HTTP.Port)
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Graceful shutdown
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 1 second.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscanll.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("Shutdown Server, waiting %d seconds before killing\n", timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	<-ctx.Done()
	log.Printf("timeout of %d seconds.\n", timeout)

	log.Println("Server exiting")
}

func Run(cfg *config.Config, version string) {
	log.Printf("Starting Assistant v%s", version)

	// Initialize database
	db, err := database.NewDatabase(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Create the combined database + markdown exporter
	exporter := exporters.NewDatabaseMarkdownExporter(
		db,
		cfg.Obsidian.VaultDir,
		cfg.Obsidian.ExportPath,
	)

	// Create auditor for saving incoming JSON requests
	auditor := audit.NewAuditor(cfg.Audit.Dir)

	router := http_controllers.NewRouter(exporter, cfg.Readwise.Token, auditor, cfg.UI.TemplatesPath, cfg.UI.StaticPath, cfg.Database.Path, cfg.Dropbox.AppKey, cfg.MoonReader.DropboxPath, cfg.MoonReader.DatabasePath, cfg.MoonReader.OutputDir, db, version)
	Serve(router, cfg)
}
