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

	"opskeeper/backend/aiengine"
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
	agentProfileVersions := skill.NewAgentProfileVersionStore(pool)
	agentProfileResolver := skill.NewAgentProfileResolver(resourceService)
	agentProfileResolver.Versions = agentProfileVersions
	contextTooling := aiengine.NewContextTooling(aiengine.ResourceServiceReader{Reader: resourceService}, connectors.AIEngineProvider())
	aiStore := aiengine.NewPostgresStore(pool)
	contextTooling.Gateway.AuditStore = aiStore
	modelBuilder := func(ctx context.Context, scopeID, providerID, modelName string, purpose aiengine.Purpose) (aiengine.ModelBuildResult, error) {
		resolved, client, err := llmService.BuildModel(ctx, scopeID, providerID, modelName, llm.Purpose(purpose))
		if err != nil {
			return aiengine.ModelBuildResult{}, err
		}
		return aiengine.ModelBuildResult{Client: client, ProviderResourceID: resolved.Provider.ResourceID, ModelName: resolved.Model.Name, Capabilities: resolved.Model.Capabilities, ContextWindowTokens: resolved.Model.ContextWindowTokens, MaxOutputTokens: resolved.Model.MaxOutputTokens}, nil
	}
	aiEngine := aiengine.NewWithContextAndStore(aiengine.NewAgentRunner(modelBuilder), contextTooling.Resolver, contextTooling.Gateway, aiStore).
		WithAgentProfileResolver(agentProfileResolver).
		WithPlanResolver(skillService)
	worker := inspection.NewWorker(store, connectorChecker{resources: resourceService, service: connectors}, inspectionExplainer{engine: aiEngine, store: store}, serviceName+":"+hostname(), cfg.InspectionLeaseDuration)
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
	engine aiengine.Engine
	store  interface {
		RecordExplanation(context.Context, string, string) error
	}
}

func (e inspectionExplainer) Explain(ctx context.Context, run inspection.Run, policy inspection.Policy, findings []inspection.Finding) error {
	if e.engine == nil {
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
	for targetID := range targets {
		if remainingTools < 1 || remainingTokens < 1 {
			if firstErr == nil {
				firstErr = fmt.Errorf("inspection AI budget exhausted")
			}
			break
		}
		var output, status string
		var toolCalls int
		var totalTokens int64
		var err error
		result, executeErr := e.engine.Execute(ctx, aiengine.Request{
			ScopeID: policy.ScopeID, Profile: aiengine.ProfileInspection, AgentProfileID: policy.AgentProfileResourceID,
			ResolvedAgentProfile: inspectionProfile(policy),
			Input:                map[string]any{"target_resource_id": targetID}, Context: aiengine.ContextRequest{ResourceIDs: []string{targetID}},
			Budget: aiengine.Budget{MaxToolCalls: remainingTools, MaxTokens: remainingTokens, Timeout: policy.Timeout},
		})
		output, toolCalls, totalTokens, status, err = result.Output, result.ToolCallCount, result.TotalTokens, string(result.Status), executeErr
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		remainingTools -= toolCalls
		remainingTokens -= totalTokens
		if status != "succeeded" {
			if firstErr == nil {
				firstErr = fmt.Errorf("inspection AIEngine execution ended with status %s", status)
			}
			continue
		}
		succeeded++
		if err := e.store.RecordExplanation(ctx, run.ID, output); err != nil {
			return err
		}
	}
	if succeeded == 0 {
		return firstErr
	}
	return nil
}

func inspectionProfile(policy inspection.Policy) *aiengine.AgentProfile {
	if policy.AgentProfileResourceID != "" {
		return nil
	}
	return &aiengine.AgentProfile{
		ResourceID: "builtin:inspection-agent", ScopeID: policy.ScopeID, Name: "巡检解释 Agent", Version: 1, Enabled: true,
		Instruction:  "你是 OpsKeeper 巡检解释专家。仅基于确定性巡检结果和授权只读上下文解释异常，明确区分事实与推断，不执行写操作，不泄露凭据。输出简洁的原因分析与建议。",
		Capabilities: []string{"text", "tool_calling", "stream"},
	}
}
func hostname() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return host
}
