package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"opskeeper/backend/config"
	"opskeeper/backend/connector"
	"opskeeper/backend/credential"
	"opskeeper/backend/inspection"
	"opskeeper/backend/logging"
	"opskeeper/backend/resource"
	"opskeeper/backend/version"
)

const serviceName = "opskeeper-worker"

func main() {
	logger := logging.NewText(os.Stdout)
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	logger, err = logging.New(os.Stdout, cfg.LogFormat)
	if err != nil {
		logger.Error("configure logging", "error", err)
		os.Exit(1)
	}
	logger = logger.With(append([]any{"service", serviceName}, version.LogAttributes()...)...)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("configure PostgreSQL client", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	encryptor, err := credential.FromEnvironment(cfg.Environment)
	if err != nil {
		logger.Error("configure credential encryption", "error", err)
		os.Exit(1)
	}
	limits := connector.DefaultLimits()
	limits.Timeout, limits.MaxConcurrent, limits.MaxResponseBytes = cfg.ConnectorTimeout, cfg.ConnectorMaxConcurrency, cfg.ConnectorMaxResponseBytes
	registry, err := connector.DefaultRegistry(limits)
	if err != nil {
		logger.Error("configure connector registry", "error", err)
		os.Exit(1)
	}
	credentials := credential.NewService(credential.NewStore(pool), encryptor)
	store := inspection.NewStore(pool)
	connectors := connector.NewService(registry, resource.NewService(resource.NewStore(pool)), credentials, connector.NewStore(pool), limits)
	worker := inspection.NewWorker(store, connectorChecker{service: connectors}, nil, serviceName+":"+hostname(), cfg.InspectionLeaseDuration)
	notifier := inspection.NotificationWorker{Store: store, Credentials: credentials}
	ticker := time.NewTicker(cfg.InspectionWorkerPollInterval)
	defer ticker.Stop()
	logger.Info("worker started", "poll_interval", cfg.InspectionWorkerPollInterval)
	for {
		claimed, runErr := worker.RunOnce(ctx)
		if runErr != nil {
			logger.Error("run inspection job", "error", runErr)
		}
		notified, notifyErr := notifier.RunOnce(ctx)
		if notifyErr != nil {
			logger.Error("deliver inspection notification", "error", notifyErr)
		}
		if claimed || notified {
			continue
		}
		select {
		case <-ctx.Done():
			logger.Info("worker stopped")
			return
		case <-ticker.C:
		}
	}
}

type connectorChecker struct {
	service interface {
		Test(context.Context, string, string) (connector.Check, error)
	}
}

func (c connectorChecker) Check(ctx context.Context, id string) ([]inspection.RuleResult, error) {
	check, err := c.service.Test(ctx, "", id)
	if err != nil {
		return nil, err
	}
	if check.Status == "succeeded" {
		return nil, nil
	}
	return []inspection.RuleResult{{TargetResourceID: id, Rule: "connector.connectivity", Severity: "critical", Weight: 50, Message: check.Message}}, nil
}
func hostname() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return host
}
