// Package cmd implements CLI subcommands for the registry server.
package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/auth"
	"github.com/yaoapp/registry/config"
	"github.com/yaoapp/registry/handlers"
	"github.com/yaoapp/registry/models"
	"github.com/yaoapp/registry/storage"
)

// RunStart executes the "start" subcommand, which initializes all resources and
// launches the HTTP server.
func RunStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)

	cfg := config.LoadFromEnv(nil)

	dbPath := fs.String("db-path", cfg.DBPath, "SQLite database file path")
	dataPath := fs.String("data-path", cfg.DataPath, "Package file storage directory")
	host := fs.String("host", cfg.Host, "Listen IP address")
	port := fs.Int("port", cfg.Port, "Listen port")
	authFile := fs.String("auth-file", cfg.AuthFile, "Authentication file path")
	maxSize := fs.Int64("max-size", cfg.MaxSize, "Maximum package size in MB")

	fs.Parse(args)

	cfg.DBPath = *dbPath
	cfg.DataPath = *dataPath
	cfg.Host = *host
	cfg.Port = *port
	cfg.AuthFile = *authFile
	cfg.MaxSize = *maxSize

	// 1. Open / create database
	db, err := models.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("init database: %v", err)
	}
	defer db.Close()
	fmt.Fprintf(os.Stderr, "Database: %s\n", cfg.DBPath)

	// 2. Create storage directory
	if err := storage.EnsureDir(cfg.DataPath); err != nil {
		log.Fatalf("create storage dir: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Storage: %s\n", cfg.DataPath)

	// 3. Load auth file
	af := auth.NewAuthFile(cfg.AuthFile)
	if err := af.Load(); err != nil {
		log.Fatalf("load auth file: %v", err)
	}
	if !af.HasUsers() {
		fmt.Fprintf(os.Stderr, "Warning: no push users configured. Run `registry user add` to add one.\n")
	}
	fmt.Fprintf(os.Stderr, "Auth: %s (%d users)\n", cfg.AuthFile, len(af.ListUsers()))

	// 4. Setup Gin and routes
	r := gin.Default()
	s := &handlers.Server{DB: db, Config: cfg, AuthFile: af}
	s.SetupRoutes(r)

	// 5. Start listening
	addr := cfg.Addr()
	fmt.Fprintf(os.Stderr, "Listening on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
