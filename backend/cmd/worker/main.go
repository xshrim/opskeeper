package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"opskeeper/backend/config"
	"opskeeper/backend/connector"
	"opskeeper/backend/credential"
	"opskeeper/backend/inspection"
	"opskeeper/backend/llm"
	"opskeeper/backend/logging"
	"opskeeper/backend/observability"
	"opskeeper/backend/operation"
	"opskeeper/backend/resource"
	"opskeeper/backend/skill"
	"opskeeper/backend/version"
)

const serviceName = "opskeeper-worker"

func main() {
	logger := logging.NewRaw(os.Stdout).With("service", serviceName)
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "kind", "error", "error_type", "configuration", "error", err)
		os.Exit(1)
	}
	logger, err = logging.New(os.Stdout, cfg.LogFormat)
	if err != nil {
		logger.Error("configure logging", "kind", "error", "error_type", "logging", "error", err)
		os.Exit(1)
	}
	logger = logger.With("service", serviceName)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	build := version.Current()
	shutdownTelemetry, err := observability.Setup(ctx, serviceName, cfg.Environment, cfg.OTLPExporterEndpoint, observability.Build{Version: build.Version, Commit: build.Commit})
	if err != nil {
		logger.Error("configure telemetry", "kind", "error", "error_type", "telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Warn("shutdown telemetry", "kind", "error", "error_type", "telemetry-shutdown", "error", err)
		}
	}()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("configure PostgreSQL client", "kind", "error", "error_type", "postgres-client", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	encryptor, err := credential.FromEnvironment(cfg.Environment)
	if err != nil {
		logger.Error("configure credential encryption", "kind", "error", "error_type", "credential-encryption", "error", err)
		os.Exit(1)
	}
	limits := connector.DefaultLimits()
	limits.Timeout, limits.MaxConcurrent, limits.MaxResponseBytes = cfg.ConnectorTimeout, cfg.ConnectorMaxConcurrency, cfg.ConnectorMaxResponseBytes
	registry, err := connector.DefaultRegistry(limits)
	if err != nil {
		logger.Error("configure connector registry", "kind", "error", "error_type", "connector-registry", "error", err)
		os.Exit(1)
	}
	credentials := credential.NewService(credential.NewStore(pool), encryptor)
	store := inspection.NewStore(pool)
	resourceService := resource.NewService(resource.NewStore(pool))
	connectors := connector.NewService(registry, resourceService, credentials, connector.NewStore(pool), limits)
	llmService := llm.NewService(llm.NewStore(pool), resourceService, credentials)
	skillService := skill.NewService(skill.NewStore(pool), resourceService)
	skillRunner := skill.NewRunner(skillService, llmService, connectors, skill.NewStore(pool))
	worker := inspection.NewWorker(store, connectorChecker{resources: resourceService, service: connectors}, inspectionExplainer{runner: skillRunner, store: store}, serviceName+":"+hostname(), cfg.InspectionLeaseDuration)
	var operationReconciler *operation.Reconciler
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OPSK_OPERATION_SUBMITTER_ENABLED")), "true") {
		operationReconciler, err = operation.NewInClusterReconciler(operation.NewStore(pool))
		if err != nil {
			logger.Warn("operation reconciler unavailable", "kind", "error", "error_type", "operation-reconciler", "error", err)
		}
	}
	notifier := inspection.NotificationWorker{Store: store, Credentials: credentials}
	ticker := time.NewTicker(cfg.InspectionWorkerPollInterval)
	defer ticker.Stop()
	logger.Info("worker started", "kind", "service-start", "poll_interval", cfg.InspectionWorkerPollInterval)
	for {
		if operationReconciler != nil {
			if _, reconcileErr := operationReconciler.RunOnce(ctx); reconcileErr != nil {
				logger.Error("reconcile operation job", "kind", "job", "error_type", "operation-reconcile", "error", reconcileErr)
			}
		}
		started := time.Now()
		claimed, runErr := worker.RunOnce(ctx)
		if runErr != nil {
			logger.Error("run inspection job", "kind", "job", "error_type", "inspection-run", "error", runErr)
			observability.RecordError(ctx, "worker", "inspection")
		}
		if claimed {
			observability.RecordTask(ctx, "inspection", taskResult(runErr), time.Since(started))
		}
		started = time.Now()
		notified, notifyErr := notifier.RunOnce(ctx)
		if notifyErr != nil {
			logger.Error("deliver inspection notification", "kind", "job", "error_type", "notification-delivery", "error", notifyErr)
			observability.RecordError(ctx, "worker", "notification")
		}
		if notified {
			observability.RecordTask(ctx, "notification", taskResult(notifyErr), time.Since(started))
		}
		if claimed || notified {
			continue
		}
		select {
		case <-ctx.Done():
			logger.Info("worker stopped", "kind", "service-stop")
			return
		case <-ticker.C:
		}
	}
}

func taskResult(err error) string {
	if err != nil {
		return "failure"
	}
	return "success"
}

type connectorChecker struct {
	resources interface {
		Get(context.Context, string) (resource.Resource, error)
	}
	service interface {
		Test(context.Context, string, string) (connector.Check, error)
		ReadKubernetes(context.Context, string, connector.KubernetesQuery) (connector.Evidence, error)
		InspectPostgreSQL(context.Context, string) (connector.Evidence, error)
		InspectRedis(context.Context, string) (connector.Evidence, error)
		InspectKafka(context.Context, string) (connector.Evidence, error)
	}
}

func (c connectorChecker) Check(ctx context.Context, id string) ([]inspection.RuleResult, error) {
	target, err := c.resources.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var evidence connector.Evidence
	switch target.Kind {
	case "PostgreSQL":
		evidence, err = c.service.InspectPostgreSQL(ctx, id)
	case "Redis":
		evidence, err = c.service.InspectRedis(ctx, id)
	case "Kafka":
		evidence, err = c.service.InspectKafka(ctx, id)
	case "Application", "BusinessApplication", "CronApplication", "Kubernetes", "KubernetesCluster":
		clusterID := target.SourceResourceID
		if clusterID == "" {
			clusterID = id
		}
		resourceName := nestedConfigString(target.Config, "kubernetes", "workload_kind")
		resourceName = strings.ToLower(resourceName)
		if resourceName == "" {
			resourceName = "deployments"
		} else if !strings.HasSuffix(resourceName, "s") {
			resourceName += "s"
		}
		workloadName := nestedConfigString(target.Config, "kubernetes", "workload_name")
		if workloadName == "" && target.Kind != "Kubernetes" && target.Kind != "KubernetesCluster" {
			workloadName = target.Name
		}
		evidence, err = c.service.ReadKubernetes(ctx, clusterID, connector.KubernetesQuery{Resource: resourceName, Namespace: configString(target.Config, "namespace"), Name: workloadName, Limit: 20})
		if err == nil && evidence.Partial {
			return []inspection.RuleResult{{TargetResourceID: id, Rule: "kubernetes.partial", Severity: "warning", Weight: 10, Message: "Kubernetes 诊断结果不完整"}}, nil
		}
	default:
		return c.connectivity(ctx, id)
	}
	if err != nil {
		return nil, err
	}
	var snapshot connector.DiagnosticSnapshot
	if err := json.Unmarshal(evidence.Data, &snapshot); err != nil {
		return nil, err
	}
	results := make([]inspection.RuleResult, 0, len(snapshot.Findings)+len(snapshot.Unavailable))
	for _, finding := range snapshot.Findings {
		weight := 20
		if finding.Severity == "critical" {
			weight = 50
		}
		results = append(results, inspection.RuleResult{TargetResourceID: id, Rule: finding.Code, Severity: finding.Severity, Weight: weight, Message: finding.Message})
	}
	for _, capability := range snapshot.Unavailable {
		results = append(results, inspection.RuleResult{TargetResourceID: id, Rule: "connector.unavailable." + capability, Severity: "warning", Weight: 10, Message: "诊断能力不可用：" + capability})
	}
	return results, nil
}

func nestedConfigString(config map[string]any, object, key string) string {
	nested, _ := config[object].(map[string]any)
	value, _ := nested[key].(string)
	return strings.TrimSpace(value)
}

func configString(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return strings.TrimSpace(value)
}

func (c connectorChecker) connectivity(ctx context.Context, id string) ([]inspection.RuleResult, error) {
	check, err := c.service.Test(ctx, "", id)
	if err != nil {
		return nil, err
	}
	if check.Status == "succeeded" {
		return nil, nil
	}
	return []inspection.RuleResult{{TargetResourceID: id, Rule: "connector.connectivity", Severity: "critical", Weight: 50, Message: check.Message}}, nil
}

type inspectionExplainer struct {
	runner *skill.Runner
	store  interface {
		RecordExplanation(context.Context, string, string) error
	}
}

func (e inspectionExplainer) Explain(ctx context.Context, run inspection.Run, policy inspection.Policy, findings []inspection.Finding) error {
	if e.runner == nil || len(policy.SkillResourceIDs) == 0 {
		return nil
	}
	targets := map[string]bool{}
	for _, finding := range findings {
		if finding.TargetResourceID != "" {
			targets[finding.TargetResourceID] = true
		}
	}
	if len(targets) == 0 {
		return nil
	}
	var firstErr error
	succeeded := 0
	remainingTools, remainingTokens := policy.MaxToolCalls, policy.MaxTokens
	for _, skillID := range policy.SkillResourceIDs {
		for targetID := range targets {
			if remainingTools < 1 || remainingTokens < 1 {
				if firstErr == nil {
					firstErr = fmt.Errorf("inspection AI budget exhausted")
				}
				break
			}
			result, err := e.runner.Run(ctx, skill.RunInput{ScopeID: policy.ScopeID, SkillResourceID: skillID, TargetResourceID: targetID, Input: map[string]any{"target_resource_id": targetID}, MaxToolCalls: remainingTools, MaxTokens: remainingTokens, Timeout: policy.Timeout})
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			remainingTools -= result.Execution.ToolCallCount
			remainingTokens -= result.Execution.TotalTokens
			if result.Execution.Status != "succeeded" {
				if firstErr == nil {
					firstErr = fmt.Errorf("inspection Skill execution ended with status %s", result.Execution.Status)
				}
				continue
			}
			succeeded++
			if err := e.store.RecordExplanation(ctx, run.ID, result.Output); err != nil {
				return err
			}
		}
	}
	if succeeded == 0 {
		return firstErr
	}
	return nil
}
func hostname() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return host
}
