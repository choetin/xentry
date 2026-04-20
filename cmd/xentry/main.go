package main

import (
	"log"
	"net/http"

	"github.com/xentry/xentry/internal/apm"
	"github.com/xentry/xentry/internal/auth"
	"github.com/xentry/xentry/internal/config"
	"github.com/xentry/xentry/internal/crash"
	"github.com/xentry/xentry/internal/db"
	logpkg "github.com/xentry/xentry/internal/log"
	"github.com/xentry/xentry/internal/org"
	"github.com/xentry/xentry/internal/project"
	"github.com/xentry/xentry/internal/release"
	"github.com/xentry/xentry/internal/router"
	"github.com/xentry/xentry/internal/symbol"
	"github.com/xentry/xentry/internal/web"
)

func main() {
	cfg := config.Load()

	store, err := db.NewSQLite(cfg.Database)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer store.Close()

	authSvc := auth.NewService(cfg.JWTSecret)
	authSvc.SetDB(store.DB())

	symSvc := symbol.NewService(store.DB(), cfg.DataDir)
	sym := symbol.NewSymbolicator(symSvc)
	crashSvc := crash.NewService(store.DB())
	crashSvc.SetSymbolicator(sym)
	crashDataDir := cfg.DataDir

	svc := &router.Services{
		DB:           store,
		Auth:         authSvc,
		Org:          org.NewService(store.DB()),
		Project:      project.NewService(store.DB()),
		Crash:        crashSvc,
		CrashDataDir: crashDataDir,
		Symbol:       symSvc,
		APM:     apm.NewService(store.DB()),
		Log:     logpkg.NewService(store.DB()),
		Release: release.NewService(store.DB()),
		Web:     web.NewHandler(store.DB(), authSvc),
	}

	r := router.New(svc)
	log.Printf("xentry listening on %s (env=%s)", cfg.Addr, cfg.Env)
	if err := http.ListenAndServe(cfg.Addr, r); err != nil {
		log.Fatal(err)
	}
}
