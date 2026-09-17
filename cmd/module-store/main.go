// Command module-store is booth-module-store's entrypoint: the Module Store catalog +
// browsing/install API (ADR 0027).
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/projectbooth/booth-module-store/internal/api"
	"github.com/projectbooth/booth-module-store/internal/auth"
	"github.com/projectbooth/booth-module-store/internal/catalog"
	"github.com/projectbooth/booth-module-store/internal/config"
	"github.com/projectbooth/booth-module-store/internal/coreclient"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	bundled, err := catalog.LoadBundled(cfg.BundledCatalogPath)
	if err != nil {
		return fmt.Errorf("loading bundled catalog: %w", err)
	}
	log.Printf("loaded %d bundled catalog entr(y/ies)", len(bundled))

	if len(cfg.RegistryURLs) == 0 {
		log.Print("no external registries configured; catalog is bundled-only (ADR 0027 default)")
	} else {
		log.Printf("configured external registries: %v", cfg.RegistryURLs)
	}

	verifier, err := auth.NewVerifier(ctx, cfg.OIDC)
	if err != nil {
		return fmt.Errorf("creating OIDC verifier: %w", err)
	}

	router := api.NewRouter(api.Deps{
		Verifier:       verifier,
		Core:           coreclient.New(cfg.CoreBaseURL),
		Bundled:        bundled,
		RegistryURLs:   cfg.RegistryURLs,
		RegistryClient: catalog.NewRegistryClient(),
	})

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("booth-module-store listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

const shutdownTimeout = 10 * time.Second
