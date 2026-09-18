package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"github.com/gopherex/courier/internal/postgres"
	"github.com/gopherex/courier/internal/service"
)

// Run opens PostgreSQL and either migrates or serves the API, UI and relay.
func Run(ctx context.Context, config *Config, migrateOnly bool) error {
	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		return errDatabaseURL
	}

	poolConfig.MaxConns = int32(config.Service.Workers) + databaseReserve //nolint:gosec // Workers is validated as 1..64.
	poolConfig.ConnConfig.ConnectTimeout = startupTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return errDatabasePool
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		return errDatabaseUnavailable
	}

	if migrateOnly {
		if migrationErr := postgres.Migrate(ctx, pool); migrationErr != nil {
			return fmt.Errorf("migrate database: %w", migrationErr)
		}

		return nil
	}

	shutdown, err := telemetry(ctx)
	if err != nil {
		return err
	}

	defer func() {
		flush, cancel := context.WithTimeout(context.WithoutCancel(ctx), startupTimeout)
		defer cancel()

		if shutdownErr := shutdown(flush); shutdownErr != nil {
			slog.ErrorContext(flush, "telemetry flush failed")
		}
	}()

	server, err := service.New(pool, &config.Service)
	if err != nil {
		return fmt.Errorf("initialize service: %w", err)
	}

	if _, err = server.ListProjects(ctx); err != nil {
		return errDatabaseSchema
	}

	handler, err := routes(config, server, pool)
	if err != nil {
		return err
	}

	return serve(ctx, config.Listen, handler, server)
}

func routes(config *Config, server *service.Service, pool *pgxpool.Pool) (http.Handler, error) {
	apiHandler, err := server.Handler()
	if err != nil {
		return nil, fmt.Errorf("initialize HTTP: %w", err)
	}

	if _, err = os.Stat(filepath.Join(config.AdminDirectory, "index.html")); err != nil {
		return nil, errAdminAssets
	}

	router := http.NewServeMux()
	router.Handle("/v1/", apiHandler)
	router.Handle("/admin/", apiHandler)
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/healthz",
		func(writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.WriteHeader(http.StatusNoContent)
		})
	router.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), time.Second)
		defer cancel()

		if pingErr := pool.Ping(ctx); pingErr != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		writer.WriteHeader(http.StatusNoContent)
	})
	router.Handle("/", http.FileServer(http.Dir(config.AdminDirectory)))

	return router, nil
}

func serve(ctx context.Context, address string, handler http.Handler, courier *service.Service) error {
	group, ctx := errgroup.WithContext(ctx)
	server := &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: headerTimeout, ReadTimeout: requestTimeout,
		WriteTimeout: requestTimeout, IdleTimeout: time.Minute, MaxHeaderBytes: maxHeaders,
	}

	group.Go(func() error {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP listener: %w", err)
		}

		return nil
	})
	group.Go(func() error {
		if err := courier.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("run relay: %w", err)
		}

		return nil
	})
	group.Go(func() error {
		<-ctx.Done()

		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdown); err != nil {
			return fmt.Errorf("HTTP shutdown: %w", err)
		}

		return nil
	})
	group.Go(func() error {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if err := courier.Cleanup(ctx); err != nil {
					slog.ErrorContext(ctx, "metadata cleanup failed")
				}
			}
		}
	})

	if err := group.Wait(); err != nil {
		return fmt.Errorf("service stopped: %w", err)
	}

	return nil
}

var (
	errDatabaseURL         = errors.New("invalid DATABASE_URL")
	errDatabasePool        = errors.New("cannot create PostgreSQL pool")
	errDatabaseUnavailable = errors.New("PostgreSQL unavailable")
	errDatabaseSchema      = errors.New("database schema unavailable; run courier migrate")
	errAdminAssets         = errors.New("admin assets missing; run make build-web")
)

const (
	startupTimeout  = 10 * time.Second
	headerTimeout   = 5 * time.Second
	requestTimeout  = 30 * time.Second
	shutdownTimeout = 35 * time.Second
	maxHeaders      = 16 * 1024
)

const databaseReserve = 8
