package action

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/foundation/secrets"
	sshclient "tars/internal/modules/action/ssh"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
	"tars/internal/modules/sshcredentials"
)

type Executor interface {
	Run(ctx context.Context, targetHost string, command string) (sshclient.Result, error)
}

type MetricsProvider interface {
	Query(ctx context.Context, query contracts.MetricsQuery) (contracts.MetricsResult, error)
}

type QueryRuntime interface {
	Query(ctx context.Context, manifest connectors.Manifest, query contracts.MetricsQuery) (contracts.MetricsResult, error)
}

type ExecutionRuntime interface {
	Execute(ctx context.Context, manifest connectors.Manifest, req contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error)
	Verify(ctx context.Context, manifest connectors.Manifest, req contracts.VerificationRequest) (contracts.VerificationResult, error)
}

type ConnectorHealthChecker interface {
	CheckHealth(ctx context.Context, manifest connectors.Manifest) (string, string, error)
}

// CapabilityRuntime is the generic runtime adapter for connector.invoke_capability.
// Implementations handle observability, delivery, MCP, skill, or any connector-type capabilities.
type CapabilityRuntime interface {
	Invoke(ctx context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error)
}

type Options struct {
	Logger                  *slog.Logger
	Metrics                 *foundationmetrics.Registry
	Executor                Executor
	MetricsProvider         MetricsProvider
	AllowedHosts            []string
	AllowedCommandPrefixes  []string
	ApprovalRouter          approvalrouting.Router
	BlockedCommandFragments []string
	AuthorizationPolicy     authorization.Evaluator
	Connectors              *connectors.Manager
	Secrets                 *secrets.Store
	QueryRuntimes           map[string]QueryRuntime
	ExecutionRuntimes       map[string]ExecutionRuntime
	CapabilityRuntimes      map[string]CapabilityRuntime
	SSHCredentials          *sshcredentials.Manager
	OutputSpoolDir          string
	MaxPersistedOutputBytes int
}

type Service struct {
	logger                  *slog.Logger
	metrics                 *foundationmetrics.Registry
	executor                Executor
	metricsProvider         MetricsProvider
	allowedHosts            map[string]struct{}
	allowedCommandPrefixes  []string
	approvalRouter          approvalrouting.Router
	blockedCommandFragments []string
	authorizationPolicy     authorization.Evaluator
	connectors              *connectors.Manager
	secrets                 *secrets.Store
	queryRuntimes           map[string]QueryRuntime
	executionRuntimes       map[string]ExecutionRuntime
	capabilityRuntimes      map[string]CapabilityRuntime
	sshCredentials          *sshcredentials.Manager
	outputSpoolDir          string
	maxPersistedOutputBytes int
}

var metricsRuntimeProtocols = map[string]struct{}{
	"prometheus_http":      {},
	"victoriametrics_http": {},
}

var executionRuntimeProtocols = map[string]struct{}{
	"jumpserver_api": {},
	"ssh_native":     {},
}

var capabilityHealthProtocols = map[string]struct{}{
	"observability_http": {},
	"log_file":           {},
	"delivery_git":       {},
	"delivery_github":    {},
	"stub":               {},
}

func NewService(opts Options) *Service {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	executor := opts.Executor
	if executor == nil {
		executor = sshclient.NewExecutor(sshclient.Config{})
	}

	return &Service{
		logger:                  logger,
		metrics:                 opts.Metrics,
		executor:                executor,
		metricsProvider:         opts.MetricsProvider,
		allowedHosts:            toSet(opts.AllowedHosts),
		allowedCommandPrefixes:  withDefaultPrefixes(opts.AllowedCommandPrefixes),
		approvalRouter:          opts.ApprovalRouter,
		blockedCommandFragments: withDefaultBlockedFragments(opts.BlockedCommandFragments),
		authorizationPolicy:     opts.AuthorizationPolicy,
		connectors:              opts.Connectors,
		secrets:                 opts.Secrets,
		queryRuntimes:           cloneQueryRuntimes(opts.QueryRuntimes),
		executionRuntimes:       cloneExecutionRuntimes(opts.ExecutionRuntimes),
		capabilityRuntimes:      cloneCapabilityRuntimes(opts.CapabilityRuntimes),
		sshCredentials:          opts.SSHCredentials,
		outputSpoolDir:          opts.OutputSpoolDir,
		maxPersistedOutputBytes: withDefaultMaxPersistedOutputBytes(opts.MaxPersistedOutputBytes),
	}
}

func (s *Service) QueryMetrics(ctx context.Context, query contracts.MetricsQuery) (contracts.MetricsResult, error) {
	runtime, manifest, ok, err := s.resolveQueryRuntime(query.ConnectorID, query.Protocol)
	if err != nil {
		return contracts.MetricsResult{}, err
	}
	selection := runtimeSelectionMode(query.ConnectorID)
	explicitConnector := strings.TrimSpace(query.ConnectorID) != ""
	connectorRuntime := runtimeMetadataForConnectorManifest("connector", selection, manifest, "", false, false, "", "")
	if ok {
		manifest.Config.Values = secrets.ResolveValues(s.secrets, manifest.Config.Values, manifest.Config.SecretRefs)
		result, err := runtime.Query(ctx, manifest, query)
		if err == nil {
			s.recordConnectorHealth(manifest.Metadata.ID, "healthy", "metrics runtime query succeeded")
			result.Runtime = connectorRuntime
			return result, nil
		}
		s.recordConnectorHealth(manifest.Metadata.ID, "degraded", fmt.Sprintf("metrics runtime query failed: %s", err.Error()))
		if explicitConnector {
			s.logger.Warn("explicit connector query runtime failed", "connector_id", manifest.Metadata.ID, "protocol", manifest.Spec.Protocol, "error", err)
			return contracts.MetricsResult{}, err
		}
		s.logger.Warn("connector query runtime failed, falling back to provider/stub metrics", "connector_id", manifest.Metadata.ID, "protocol", manifest.Spec.Protocol, "error", err)
	}
	if s.metricsProvider != nil {
		result, err := s.metricsProvider.Query(ctx, query)
		if err == nil {
			result.Runtime = runtimeMetadataForFallback("legacy_provider", selection, query.ConnectorID, query.ConnectorType, query.ConnectorVendor, firstNonEmpty(query.Protocol, manifest.Spec.Protocol, "victoriametrics_http"), "", true, ok, queryRuntimeFallbackReason(ok, manifest), "stub")
			if result.Runtime != nil {
				result.Runtime.FallbackUsed = true
			}
			return result, nil
		}
		s.logger.Warn("metrics provider query failed, falling back to stub metrics", "service", query.Service, "host", query.Host, "error", err)
	}

	return contracts.MetricsResult{Series: []map[string]interface{}{
		{
			"service": query.Service,
			"host":    query.Host,
			"value":   1,
			"source":  "stub",
		},
	}, Runtime: runtimeMetadataForFallback("stub", selection, query.ConnectorID, query.ConnectorType, query.ConnectorVendor, firstNonEmpty(query.Protocol, manifest.Spec.Protocol, "victoriametrics_http"), "", true, true, queryRuntimeFallbackReason(ok, manifest), "stub")}, nil
}

func (s *Service) resolveQueryRuntime(connectorID string, protocol string) (QueryRuntime, connectors.Manifest, bool, error) {
	if s == nil || s.connectors == nil {
		return nil, connectors.Manifest{}, false, nil
	}
	manifest, ok, err := s.resolveRuntimeManifest(connectorID, "metrics", protocol, metricsRuntimeProtocols)
	if err != nil {
		return nil, connectors.Manifest{}, false, err
	}
	if !ok {
		return nil, connectors.Manifest{}, false, nil
	}
	runtimeProtocol := strings.TrimSpace(protocol)
	if runtimeProtocol == "" {
		runtimeProtocol = strings.TrimSpace(manifest.Spec.Protocol)
	}
	runtime, ok := s.queryRuntimes[runtimeProtocol]
	if !ok {
		if strings.TrimSpace(connectorID) != "" {
			return nil, connectors.Manifest{}, false, fmt.Errorf("unsupported metrics connector protocol %s", runtimeProtocol)
		}
		return nil, connectors.Manifest{}, false, nil
	}
	return runtime, manifest, true, nil
}

func (s *Service) ExecuteApproved(ctx context.Context, req contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
	runtime, manifest, ok, err := s.resolveExecutionRuntime(req.ConnectorID, req.Protocol)
	if err != nil {
		return contracts.ExecutionResult{}, err
	}
	if ok {
		manifest.Config.Values = secrets.ResolveValues(s.secrets, manifest.Config.Values, manifest.Config.SecretRefs)
		result, err := runtime.Execute(ctx, manifest, req)
		if err == nil {
			s.recordConnectorHealth(manifest.Metadata.ID, "healthy", "execution runtime completed successfully")
			result.ExecutionID = req.ExecutionID
			result.SessionID = req.SessionID
			result.ConnectorID = manifest.Metadata.ID
			result.Protocol = manifest.Spec.Protocol
			result.ExecutionMode = firstNonEmpty(req.ExecutionMode, connectors.DefaultExecutionMode(manifest.Spec.Protocol))
			result.Runtime = runtimeMetadataForConnectorManifest("connector", runtimeSelectionMode(req.ConnectorID), manifest, result.ExecutionMode, true, false, "", "")
			if result.Output != "" && (result.OutputBytes == 0 || result.OutputRef == "") {
				outputRef, persistErr := s.persistOutput(req.ExecutionID, result.Output)
				if persistErr != nil {
					s.logger.Warn("persist execution output failed", "execution_id", req.ExecutionID, "error", persistErr)
				} else if result.OutputRef == "" {
					result.OutputRef = outputRef
				}
				outputPreview, outputBytes, outputTruncated := prepareExecutionOutput(result.Output, s.maxPersistedOutputBytes)
				if result.OutputPreview == "" {
					result.OutputPreview = outputPreview
				}
				result.OutputBytes = outputBytes
				result.OutputTruncated = outputTruncated
			}
			return result, nil
		}
		s.recordConnectorHealth(manifest.Metadata.ID, "degraded", fmt.Sprintf("execution runtime failed: %s", err.Error()))
		return result, err
	}
	if err := s.validateRequest(req); err != nil {
		if s.metrics != nil {
			s.metrics.RecordComponentResult("ssh", "error", err.Error())
		}
		return contracts.ExecutionResult{}, err
	}

	result, err := s.executor.Run(ctx, req.TargetHost, req.Command)
	if err != nil && !errors.Is(err, sshclient.ErrRemoteCommandFailed) && !errors.Is(err, sshclient.ErrCommandTimedOut) {
		if s.metrics != nil {
			s.metrics.RecordComponentResult("ssh", "error", err.Error())
		}
		return contracts.ExecutionResult{}, err
	}

	outputRef, persistErr := s.persistOutput(req.ExecutionID, result.Output)
	if persistErr != nil {
		s.logger.Warn("persist execution output failed", "execution_id", req.ExecutionID, "error", persistErr)
	}
	outputPreview, outputBytes, outputTruncated := prepareExecutionOutput(result.Output, s.maxPersistedOutputBytes)

	status := "completed"
	switch {
	case errors.Is(err, sshclient.ErrCommandTimedOut) || result.TimedOut:
		status = "timeout"
	case result.ExitCode != 0:
		status = "failed"
	}

	s.logger.Info(
		"execution finished",
		"execution_id", req.ExecutionID,
		"session_id", req.SessionID,
		"target_host", req.TargetHost,
		"status", status,
		"exit_code", result.ExitCode,
		"output_ref", outputRef,
	)
	if s.metrics != nil {
		s.metrics.IncExecution(status)
		if outputTruncated {
			s.metrics.IncExecutionOutputTruncated()
		}
		s.metrics.RecordComponentResult("ssh", status, fmt.Sprintf("target=%s", req.TargetHost))
	}

	return contracts.ExecutionResult{
		ExecutionID:     req.ExecutionID,
		SessionID:       req.SessionID,
		Status:          status,
		ConnectorID:     req.ConnectorID,
		Protocol:        firstNonEmpty(req.Protocol, "ssh"),
		ExecutionMode:   firstNonEmpty(req.ExecutionMode, "ssh"),
		Runtime:         runtimeMetadataForFallback("ssh", runtimeSelectionMode(req.ConnectorID), req.ConnectorID, req.ConnectorType, req.ConnectorVendor, firstNonEmpty(req.Protocol, "ssh"), firstNonEmpty(req.ExecutionMode, "ssh"), true, true, executionRuntimeFallbackReason(ok, manifest), "ssh"),
		ExitCode:        result.ExitCode,
		OutputRef:       outputRef,
		OutputPreview:   outputPreview,
		OutputBytes:     outputBytes,
		OutputTruncated: outputTruncated,
	}, nil
}

func (s *Service) VerifyExecution(ctx context.Context, req contracts.VerificationRequest) (contracts.VerificationResult, error) {
	runtime, manifest, ok, err := s.resolveExecutionRuntime(req.ConnectorID, req.Protocol)
	if err != nil {
		return contracts.VerificationResult{}, err
	}
	if ok {
		manifest.Config.Values = secrets.ResolveValues(s.secrets, manifest.Config.Values, manifest.Config.SecretRefs)
		result, err := runtime.Verify(ctx, manifest, req)
		if err != nil {
			s.recordConnectorHealth(manifest.Metadata.ID, "degraded", fmt.Sprintf("execution verification failed: %s", err.Error()))
			result.Runtime = runtimeMetadataForConnectorManifest("connector", runtimeSelectionMode(req.ConnectorID), manifest, firstNonEmpty(req.ExecutionMode, connectors.DefaultExecutionMode(manifest.Spec.Protocol)), true, false, "", "")
			return result, err
		}
		if strings.EqualFold(strings.TrimSpace(result.Status), "success") || strings.EqualFold(strings.TrimSpace(result.Status), "skipped") {
			s.recordConnectorHealth(manifest.Metadata.ID, "healthy", firstNonEmpty(strings.TrimSpace(result.Summary), "execution verification completed"))
		} else {
			s.recordConnectorHealth(manifest.Metadata.ID, "degraded", firstNonEmpty(strings.TrimSpace(result.Summary), "execution verification failed"))
		}
		result.Runtime = runtimeMetadataForConnectorManifest("connector", runtimeSelectionMode(req.ConnectorID), manifest, firstNonEmpty(req.ExecutionMode, connectors.DefaultExecutionMode(manifest.Spec.Protocol)), true, false, "", "")
		return result, nil
	}
	checkedAt := time.Now().UTC()
	sshRuntime := runtimeMetadataForFallback("ssh", runtimeSelectionMode(req.ConnectorID), req.ConnectorID, "execution", "", firstNonEmpty(req.Protocol, "ssh"), firstNonEmpty(req.ExecutionMode, "ssh"), true, true, executionRuntimeFallbackReason(ok, manifest), "ssh")
	if strings.TrimSpace(req.TargetHost) == "" {
		return contracts.VerificationResult{
			SessionID:   req.SessionID,
			ExecutionID: req.ExecutionID,
			Status:      "skipped",
			Summary:     "verification skipped: target host is empty",
			Runtime:     sshRuntime,
			CheckedAt:   checkedAt,
			Details: map[string]interface{}{
				"mode": "skipped",
			},
		}, nil
	}

	service := strings.TrimSpace(req.Service)
	if service == "" {
		return contracts.VerificationResult{
			SessionID:   req.SessionID,
			ExecutionID: req.ExecutionID,
			Status:      "skipped",
			Summary:     "verification skipped: no service hint available",
			Runtime:     sshRuntime,
			CheckedAt:   checkedAt,
			Details: map[string]interface{}{
				"mode": "skipped",
			},
		}, nil
	}

	serviceCandidates := VerificationServiceCandidates(service)
	command := fmt.Sprintf("systemctl is-active %s", service)
	if err := s.validateRequest(contracts.ApprovedExecutionRequest{
		ExecutionID: req.ExecutionID,
		SessionID:   req.SessionID,
		TargetHost:  req.TargetHost,
		Command:     command,
		Service:     req.Service,
	}); err != nil {
		if s.metrics != nil {
			s.metrics.RecordComponentResult("ssh", "error", err.Error())
		}
		return contracts.VerificationResult{
			SessionID:   req.SessionID,
			ExecutionID: req.ExecutionID,
			Status:      "failed",
			Summary:     fmt.Sprintf("verification failed: %s", err.Error()),
			Runtime:     sshRuntime,
			CheckedAt:   checkedAt,
			Details: map[string]interface{}{
				"command": command,
				"error":   err.Error(),
			},
		}, nil
	}

	var (
		lastResult  sshclient.Result
		lastErr     error
		matched     string
		attempted   []string
		lastCommand string
		lastOutput  string
	)
	for _, candidate := range serviceCandidates {
		lastCommand = fmt.Sprintf("systemctl is-active %s", candidate)
		attempted = append(attempted, candidate)
		lastResult, lastErr = s.executor.Run(ctx, req.TargetHost, lastCommand)
		lastOutput = strings.TrimSpace(lastResult.Output)
		if lastErr == nil && strings.HasPrefix(lastOutput, "active") {
			matched = candidate
			break
		}
		if errors.Is(lastErr, sshclient.ErrCommandTimedOut) || lastResult.TimedOut {
			break
		}
	}

	details := map[string]interface{}{
		"command":            lastCommand,
		"exit_code":          lastResult.ExitCode,
		"output":             truncateVerificationOutput(lastOutput, 240),
		"service":            service,
		"service_candidates": attempted,
	}
	if matched != "" {
		details["matched_service"] = matched
		if s.metrics != nil {
			s.metrics.RecordComponentResult("ssh", "success", fmt.Sprintf("verification passed for %s", matched))
		}
		return contracts.VerificationResult{
			SessionID:   req.SessionID,
			ExecutionID: req.ExecutionID,
			Status:      "success",
			Summary:     fmt.Sprintf("verification passed: %s is active", matched),
			Runtime:     sshRuntime,
			CheckedAt:   checkedAt,
			Details:     details,
		}, nil
	}
	if errors.Is(lastErr, sshclient.ErrCommandTimedOut) || lastResult.TimedOut {
		details["error"] = "verification command timed out"
		if s.metrics != nil {
			s.metrics.RecordComponentResult("ssh", "timeout", fmt.Sprintf("verification timed out for %s", service))
		}
		return contracts.VerificationResult{
			SessionID:   req.SessionID,
			ExecutionID: req.ExecutionID,
			Status:      "failed",
			Summary:     fmt.Sprintf("verification failed: timed out checking %s", service),
			Runtime:     sshRuntime,
			CheckedAt:   checkedAt,
			Details:     details,
		}, nil
	}
	if lastErr != nil {
		details["error"] = lastErr.Error()
	}
	summary := fmt.Sprintf("verification failed: %s is not active", service)
	if lastOutput == "" && lastErr != nil {
		summary = fmt.Sprintf("verification failed: could not confirm %s status", service)
	}
	if s.metrics != nil {
		s.metrics.RecordComponentResult("ssh", "failed", summary)
	}
	return contracts.VerificationResult{
		SessionID:   req.SessionID,
		ExecutionID: req.ExecutionID,
		Status:      "failed",
		Summary:     summary,
		Runtime:     sshRuntime,
		CheckedAt:   checkedAt,
		Details:     details,
	}, nil
}

func (s *Service) resolveExecutionRuntime(connectorID string, protocol string) (ExecutionRuntime, connectors.Manifest, bool, error) {
	if s == nil || s.connectors == nil {
		return nil, connectors.Manifest{}, false, nil
	}
	manifest, ok, err := s.resolveRuntimeManifest(connectorID, "execution", protocol, executionRuntimeProtocols)
	if err != nil {
		return nil, connectors.Manifest{}, false, err
	}
	if !ok {
		return nil, connectors.Manifest{}, false, nil
	}
	runtimeProtocol := strings.TrimSpace(protocol)
	if runtimeProtocol == "" {
		runtimeProtocol = strings.TrimSpace(manifest.Spec.Protocol)
	}
	runtime, ok := s.executionRuntimes[runtimeProtocol]
	if !ok {
		if strings.TrimSpace(connectorID) != "" {
			return nil, connectors.Manifest{}, false, fmt.Errorf("unsupported execution connector protocol %s", runtimeProtocol)
		}
		return nil, connectors.Manifest{}, false, nil
	}
	return runtime, manifest, true, nil
}

func (s *Service) resolveRuntimeManifest(connectorID string, expectedType string, protocol string, supportedProtocols map[string]struct{}) (connectors.Manifest, bool, error) {
	if s == nil || s.connectors == nil {
		return connectors.Manifest{}, false, nil
	}
	var (
		manifest connectors.Manifest
		err      error
	)
	if strings.TrimSpace(connectorID) != "" {
		manifest, err = connectors.ResolveRuntimeManifest(s.connectors, connectorID, expectedType, protocol, supportedProtocols)
		if err != nil {
			return connectors.Manifest{}, false, err
		}
		return manifest, true, nil
	}
	manifest, ok := connectors.SelectHealthyRuntimeManifest(s.connectors, expectedType, protocol, supportedProtocols)
	if !ok {
		return connectors.Manifest{}, false, nil
	}
	return manifest, true, nil
}

func (s *Service) CheckConnectorHealth(ctx context.Context, connectorID string) (connectors.LifecycleState, error) {
	if s == nil || s.connectors == nil {
		return connectors.LifecycleState{}, connectors.ErrConfigPathNotSet
	}
	manifest, err := connectors.ResolveRuntimeManifest(s.connectors, connectorID, "", "", nil)
	if err != nil {
		return connectors.LifecycleState{}, err
	}
	compatibility := connectors.CompatibilityReportForManifest(manifest)
	status := "healthy"
	summary := "connector health check succeeded"
	checkedAt := time.Now().UTC()
	compatibility.CheckedAt = checkedAt
	fallbackHealth := connectors.HealthStatusForManifest(manifest, compatibility, checkedAt)
	if runtime, ok := s.queryRuntimes[strings.TrimSpace(manifest.Spec.Protocol)]; ok {
		if checker, ok := runtime.(ConnectorHealthChecker); ok {
			manifest.Config.Values = secrets.ResolveValues(s.secrets, manifest.Config.Values, manifest.Config.SecretRefs)
			status, summary, err = checker.CheckHealth(ctx, manifest)
			if err != nil {
				summary = firstNonEmpty(strings.TrimSpace(summary), err.Error())
			}
			if compatibility.Compatible {
				summary = firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary)
			} else {
				status = connectors.NormalizeHealthStatus(status)
				summary = appendCompatibilitySummary(firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary), fallbackHealth.Summary)
			}
			state, err := s.connectors.RecordHealth(manifest.Metadata.ID, status, summary, checkedAt)
			if err != nil {
				return connectors.LifecycleState{}, err
			}
			state.Compatibility = compatibility
			return state, nil
		}
	}
	if runtime, ok := s.executionRuntimes[strings.TrimSpace(manifest.Spec.Protocol)]; ok {
		if checker, ok := runtime.(ConnectorHealthChecker); ok {
			manifest.Config.Values = secrets.ResolveValues(s.secrets, manifest.Config.Values, manifest.Config.SecretRefs)
			status, summary, err = checker.CheckHealth(ctx, manifest)
			if err != nil {
				summary = firstNonEmpty(strings.TrimSpace(summary), err.Error())
			}
			if compatibility.Compatible {
				summary = firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary)
			} else {
				status = connectors.NormalizeHealthStatus(status)
				summary = appendCompatibilitySummary(firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary), fallbackHealth.Summary)
			}
			state, err := s.connectors.RecordHealth(manifest.Metadata.ID, status, summary, checkedAt)
			if err != nil {
				return connectors.LifecycleState{}, err
			}
			state.Compatibility = compatibility
			return state, nil
		}
	}
	if runtime, ok := s.capabilityRuntimes[capabilityRuntimeKey(manifest)]; ok {
		if checker, ok := runtime.(ConnectorHealthChecker); ok {
			manifest.Config.Values = secrets.ResolveValues(s.secrets, manifest.Config.Values, manifest.Config.SecretRefs)
			status, summary, err = checker.CheckHealth(ctx, manifest)
			if err != nil {
				summary = firstNonEmpty(strings.TrimSpace(summary), err.Error())
			}
			if compatibility.Compatible {
				summary = firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary)
			} else {
				status = connectors.NormalizeHealthStatus(status)
				summary = appendCompatibilitySummary(firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary), fallbackHealth.Summary)
			}
			state, err := s.connectors.RecordHealth(manifest.Metadata.ID, status, summary, checkedAt)
			if err != nil {
				return connectors.LifecycleState{}, err
			}
			state.Compatibility = compatibility
			return state, nil
		}
	}
	state, err := s.connectors.RecordHealth(manifest.Metadata.ID, fallbackHealth.Status, fallbackHealth.Summary, checkedAt)
	if err != nil {
		return connectors.LifecycleState{}, err
	}
	state.Compatibility = compatibility
	return state, nil
}

func (s *Service) CheckManifestHealth(ctx context.Context, manifest connectors.Manifest) (connectors.LifecycleState, error) {
	if s == nil {
		return connectors.LifecycleState{}, connectors.ErrConfigPathNotSet
	}
	if err := connectors.ValidateManifest(manifest); err != nil {
		return connectors.LifecycleState{}, err
	}

	checkedAt := time.Now().UTC()
	compatibility := connectors.CompatibilityReportForManifest(manifest)
	compatibility.CheckedAt = checkedAt

	probeManifest := manifest
	// Draft probes should validate real connectivity even if the user plans to save disabled.
	probeManifest.Disabled = false
	if err := connectors.ValidateRuntimeConfig(probeManifest, probeManifest.Spec.Protocol); err != nil {
		return connectors.LifecycleState{}, err
	}

	state := connectors.LifecycleState{
		ConnectorID:    manifest.Metadata.ID,
		DisplayName:    firstNonEmpty(manifest.Metadata.DisplayName, manifest.Metadata.Name, manifest.Metadata.ID),
		CurrentVersion: manifest.Metadata.Version,
		Enabled:        manifest.Enabled(),
		InstalledAt:    checkedAt,
		UpdatedAt:      checkedAt,
		Runtime:        connectors.RuntimeMetadataForManifest(manifest),
		Compatibility:  compatibility,
	}

	fallbackHealth := connectors.HealthStatusForManifest(probeManifest, compatibility, checkedAt)
	checker, ok := s.resolveDraftHealthChecker(probeManifest)
	if !ok {
		return connectors.LifecycleState{}, fmt.Errorf("%w: protocol %s does not support draft health probe", connectors.ErrConnectorRuntimeUnsupported, strings.TrimSpace(manifest.Spec.Protocol))
	}

	probeManifest.Config.Values = secrets.ResolveValues(s.secrets, probeManifest.Config.Values, probeManifest.Config.SecretRefs)
	status, summary, err := checker.CheckHealth(ctx, probeManifest)
	if err != nil {
		// Use status from CheckHealth (e.g., "unhealthy" for connection failures)
		// If CheckHealth didn't set a status, default to "unhealthy"
		if status == "" {
			status = "unhealthy"
		}
		summary = firstNonEmpty(strings.TrimSpace(summary), err.Error())
	}
	if compatibility.Compatible {
		summary = firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary)
	} else {
		// Compatibility check failed - mark as unhealthy
		status = "unhealthy"
		summary = appendCompatibilitySummary(firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary), fallbackHealth.Summary)
	}

	state.Health = connectors.HealthStatus{
		Status:           firstNonEmpty(strings.TrimSpace(status), fallbackHealth.Status),
		CredentialStatus: fallbackHealth.CredentialStatus,
		Summary:          firstNonEmpty(strings.TrimSpace(summary), fallbackHealth.Summary),
		CheckedAt:        checkedAt,
	}
	state.HealthHistory = []connectors.HealthStatus{state.Health}
	return state, nil
}

func (s *Service) recordConnectorHealth(id string, status string, summary string) {
	if s == nil || s.connectors == nil || strings.TrimSpace(id) == "" {
		return
	}
	if _, err := s.connectors.RecordHealth(id, status, summary, time.Now().UTC()); err != nil {
		s.logger.Warn("record connector health failed", "connector_id", id, "error", err)
	}
}

func appendCompatibilitySummary(runtimeSummary string, compatibilitySummary string) string {
	runtimeSummary = strings.TrimSpace(runtimeSummary)
	compatibilitySummary = strings.TrimSpace(compatibilitySummary)
	switch {
	case runtimeSummary == "":
		return compatibilitySummary
	case compatibilitySummary == "":
		return runtimeSummary
	case runtimeSummary == compatibilitySummary:
		return runtimeSummary
	case strings.Contains(runtimeSummary, compatibilitySummary):
		return runtimeSummary
	default:
		return runtimeSummary + "; " + compatibilitySummary
	}
}

func (s *Service) resolveDraftHealthChecker(manifest connectors.Manifest) (ConnectorHealthChecker, bool) {
	if s == nil {
		return nil, false
	}
	protocol := strings.TrimSpace(manifest.Spec.Protocol)
	if runtime, ok := s.queryRuntimes[protocol]; ok {
		if checker, ok := runtime.(ConnectorHealthChecker); ok {
			return checker, true
		}
	}
	if runtime, ok := s.executionRuntimes[protocol]; ok {
		if checker, ok := runtime.(ConnectorHealthChecker); ok {
			return checker, true
		}
	}
	if runtime, ok := s.capabilityRuntimes[capabilityRuntimeKey(manifest)]; ok {
		if checker, ok := runtime.(ConnectorHealthChecker); ok {
			return checker, true
		}
	}
	return nil, false
}

func (s *Service) validateRequest(req contracts.ApprovedExecutionRequest) error {
	command := strings.TrimSpace(req.Command)
	if req.ExecutionID == "" || req.SessionID == "" || req.TargetHost == "" || command == "" {
		return fmt.Errorf("execution request is incomplete")
	}

	if len(s.allowedHosts) > 0 {
		if _, ok := s.allowedHosts[req.TargetHost]; !ok {
			return fmt.Errorf("target host %s is not in allowlist", req.TargetHost)
		}
	}

	if err := validateBlockedFragments(command, s.blockedCommandFragments); err != nil {
		return err
	}

	if s.authorizationPolicy != nil {
		decision := s.authorizationPolicy.EvaluateSSHCommand(authorization.SSHCommandInput{
			Command: command,
			Service: req.Service,
			Host:    req.TargetHost,
		})
		switch decision.Action {
		case authorization.ActionDirectExecute, authorization.ActionRequireApproval:
			return nil
		case authorization.ActionSuggestOnly:
			return fmt.Errorf("command requires manual handling by authorization policy")
		case authorization.ActionDeny:
			return fmt.Errorf("command denied by authorization policy")
		default:
			return fmt.Errorf("command blocked by unknown authorization action")
		}
	}

	if err := validateCommand(command, s.allowedCommandPrefixes, s.blockedCommandFragments, req.Service, s.serviceCommandAllowlist()); err != nil {
		return err
	}

	return nil
}

func (s *Service) persistOutput(executionID string, output string) (string, error) {
	if strings.TrimSpace(s.outputSpoolDir) == "" || output == "" {
		return "", nil
	}

	if err := os.MkdirAll(s.outputSpoolDir, 0o750); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%s-%s.log", executionID, time.Now().UTC().Format("20060102T150405Z"))
	path := filepath.Join(s.outputSpoolDir, filename)
	if err := os.WriteFile(path, []byte(output), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func toSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}

	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out[trimmed] = struct{}{}
		}
	}
	return out
}

func cloneQueryRuntimes(input map[string]QueryRuntime) map[string]QueryRuntime {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]QueryRuntime, len(input))
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || value == nil {
			continue
		}
		out[trimmed] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneExecutionRuntimes(input map[string]ExecutionRuntime) map[string]ExecutionRuntime {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]ExecutionRuntime, len(input))
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || value == nil {
			continue
		}
		out[trimmed] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func runtimeSelectionMode(connectorID string) string {
	if strings.TrimSpace(connectorID) != "" {
		return "explicit_connector"
	}
	return "auto_selector"
}

func runtimeMetadataForConnectorManifest(runtime string, selection string, manifest connectors.Manifest, executionMode string, fallbackEnabled bool, fallbackUsed bool, fallbackReason string, fallbackTarget string) *contracts.RuntimeMetadata {
	if strings.TrimSpace(manifest.Metadata.ID) == "" && strings.TrimSpace(manifest.Spec.Protocol) == "" {
		return nil
	}
	return &contracts.RuntimeMetadata{
		Runtime:         runtime,
		Selection:       selection,
		ConnectorID:     strings.TrimSpace(manifest.Metadata.ID),
		ConnectorType:   strings.TrimSpace(manifest.Spec.Type),
		ConnectorVendor: strings.TrimSpace(manifest.Metadata.Vendor),
		Protocol:        strings.TrimSpace(manifest.Spec.Protocol),
		ExecutionMode:   firstNonEmpty(executionMode, connectors.DefaultExecutionMode(manifest.Spec.Protocol)),
		FallbackEnabled: fallbackEnabled,
		FallbackUsed:    fallbackUsed,
		FallbackReason:  strings.TrimSpace(fallbackReason),
		FallbackTarget:  strings.TrimSpace(fallbackTarget),
	}
}

func runtimeMetadataForFallback(runtime string, selection string, connectorID string, connectorType string, connectorVendor string, protocol string, executionMode string, fallbackEnabled bool, fallbackUsed bool, fallbackReason string, fallbackTarget string) *contracts.RuntimeMetadata {
	return &contracts.RuntimeMetadata{
		Runtime:         strings.TrimSpace(runtime),
		Selection:       selection,
		ConnectorID:     strings.TrimSpace(connectorID),
		ConnectorType:   strings.TrimSpace(connectorType),
		ConnectorVendor: strings.TrimSpace(connectorVendor),
		Protocol:        strings.TrimSpace(protocol),
		ExecutionMode:   strings.TrimSpace(executionMode),
		FallbackEnabled: fallbackEnabled,
		FallbackUsed:    fallbackUsed,
		FallbackReason:  strings.TrimSpace(fallbackReason),
		FallbackTarget:  strings.TrimSpace(fallbackTarget),
	}
}

func queryRuntimeFallbackReason(connectorSelected bool, manifest connectors.Manifest) string {
	if connectorSelected {
		return "connector_runtime_failed"
	}
	if strings.TrimSpace(manifest.Metadata.ID) != "" {
		return "connector_runtime_unavailable"
	}
	return "no_compatible_connector_selected"
}

func executionRuntimeFallbackReason(connectorSelected bool, manifest connectors.Manifest) string {
	if connectorSelected {
		return "connector_runtime_unavailable"
	}
	if strings.TrimSpace(manifest.Metadata.ID) != "" {
		return "connector_runtime_unavailable"
	}
	return "no_compatible_connector_selected"
}

func withDefaultMaxPersistedOutputBytes(value int) int {
	if value <= 0 {
		return 262144
	}
	return value
}

func prepareExecutionOutput(output string, maxBytes int) (string, int64, bool) {
	totalBytes := int64(len([]byte(output)))
	if output == "" || maxBytes <= 0 {
		return output, totalBytes, false
	}
	preview, truncated := truncateUTF8ByBytes(output, maxBytes)
	return preview, totalBytes, truncated
}

func truncateUTF8ByBytes(input string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(input) == 0 {
		return "", len(input) > 0
	}
	if len([]byte(input)) <= maxBytes {
		return input, false
	}

	lastBoundary := 0
	for index := range input {
		if index > maxBytes {
			break
		}
		lastBoundary = index
	}
	if lastBoundary == 0 {
		return "", true
	}
	return input[:lastBoundary], true
}

func truncateVerificationOutput(input string, maxLen int) string {
	if maxLen <= 0 || len(input) <= maxLen {
		return input
	}
	return input[:maxLen]
}

func (s *Service) serviceCommandAllowlist() map[string][]string {
	if s == nil || s.approvalRouter == nil {
		return map[string][]string{}
	}
	return s.approvalRouter.ServiceCommandAllowlist()
}

func (s *Service) InvokeCapability(ctx context.Context, req contracts.CapabilityRequest) (contracts.CapabilityResult, error) {
	connectorID := strings.TrimSpace(req.ConnectorID)
	capabilityID := strings.TrimSpace(req.CapabilityID)

	if connectorID == "" {
		return contracts.CapabilityResult{Status: "failed", Error: "connector_id is required"}, fmt.Errorf("connector_id is required for invoke_capability")
	}
	if capabilityID == "" {
		return contracts.CapabilityResult{Status: "failed", Error: "capability_id is required"}, fmt.Errorf("capability_id is required for invoke_capability")
	}

	if s.connectors == nil {
		return contracts.CapabilityResult{Status: "failed", Error: "connector manager not available"}, fmt.Errorf("connector manager not available")
	}

	// Resolve connector manifest
	manifest, err := connectors.ResolveRuntimeManifest(s.connectors, connectorID, "", "", nil)
	if err != nil {
		return contracts.CapabilityResult{Status: "failed", Error: err.Error()}, err
	}

	// Verify capability exists on connector and check invocable
	capability, found := findCapability(manifest, capabilityID)
	if !found {
		return contracts.CapabilityResult{
			Status: "failed",
			Error:  fmt.Sprintf("capability %q not found on connector %q", capabilityID, connectorID),
		}, fmt.Errorf("capability %q not found on connector %q", capabilityID, connectorID)
	}

	// Authorization check
	if s.authorizationPolicy != nil && !req.SkipAuthorization {
		decision := s.authorizationPolicy.EvaluateCapability(authorization.CapabilityInput{
			ConnectorID:  connectorID,
			CapabilityID: capabilityID,
			ReadOnly:     capability.ReadOnly,
			Source:       connectorSourceType(manifest),
		})
		switch decision.Action {
		case authorization.ActionDirectExecute:
			// proceed
		case authorization.ActionRequireApproval:
			return contracts.CapabilityResult{
				Status: "pending_approval",
				Error:  "capability requires approval",
				Metadata: map[string]interface{}{
					"rule_id":    decision.RuleID,
					"matched_by": decision.MatchedBy,
				},
			}, nil
		case authorization.ActionDeny:
			return contracts.CapabilityResult{
				Status: "denied",
				Error:  fmt.Sprintf("capability %q denied by policy (rule: %s)", capabilityID, decision.RuleID),
				Metadata: map[string]interface{}{
					"rule_id":    decision.RuleID,
					"matched_by": decision.MatchedBy,
				},
			}, nil
		default:
			return contracts.CapabilityResult{
				Status: "denied",
				Error:  "capability blocked by unknown authorization action",
			}, fmt.Errorf("capability blocked by unknown authorization action")
		}
	}

	// Resolve capability runtime
	runtimeKey := capabilityRuntimeKey(manifest)
	runtime, ok := s.capabilityRuntimes[runtimeKey]
	if !ok {
		// Try normalized connector type as fallback key.
		runtime, ok = s.capabilityRuntimes[connectors.NormalizeCapabilityConnectorType(manifest.Spec.Type)]
	}
	if !ok {
		// Try raw connector type as a final fallback for older registrations.
		runtime, ok = s.capabilityRuntimes[strings.ToLower(strings.TrimSpace(manifest.Spec.Type))]
	}
	if !ok {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   fmt.Sprintf("no capability runtime for connector type=%s protocol=%s", manifest.Spec.Type, manifest.Spec.Protocol),
			Runtime: runtimeMetadataForConnectorManifest("capability", "explicit_connector", manifest, "", false, false, "", ""),
		}, fmt.Errorf("no capability runtime for connector type=%s protocol=%s", manifest.Spec.Type, manifest.Spec.Protocol)
	}

	manifest.Config.Values = secrets.ResolveValues(s.secrets, manifest.Config.Values, manifest.Config.SecretRefs)
	result, err := runtime.Invoke(ctx, manifest, capabilityID, req.Params)
	if err != nil {
		s.recordConnectorHealth(connectorID, "degraded", fmt.Sprintf("capability invocation failed: %s", err.Error()))
		if result.Runtime == nil {
			result.Runtime = runtimeMetadataForConnectorManifest("capability", "explicit_connector", manifest, "", false, false, "", "")
		}
		return result, err
	}

	s.recordConnectorHealth(connectorID, "healthy", "capability invocation succeeded")
	if result.Runtime == nil {
		result.Runtime = runtimeMetadataForConnectorManifest("capability", "explicit_connector", manifest, "", false, false, "", "")
	}
	return result, nil
}

func (s *Service) InvokeApprovedCapability(ctx context.Context, req contracts.ApprovedCapabilityRequest) (contracts.CapabilityResult, error) {
	return s.InvokeCapability(ctx, contracts.CapabilityRequest{
		ConnectorID:       req.ConnectorID,
		CapabilityID:      req.CapabilityID,
		Params:            cloneCapabilityParams(req.Params),
		SessionID:         req.SessionID,
		Caller:            firstNonEmpty(req.RequestedBy, "approved_capability"),
		SkipAuthorization: true,
	})
}

func cloneCapabilityParams(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func findCapability(manifest connectors.Manifest, capabilityID string) (connectors.Capability, bool) {
	capID := strings.ToLower(strings.TrimSpace(capabilityID))
	for _, cap := range manifest.Spec.Capabilities {
		if strings.ToLower(strings.TrimSpace(cap.ID)) == capID {
			return cap, true
		}
	}
	return connectors.Capability{}, false
}

func connectorSourceType(manifest connectors.Manifest) string {
	switch connectors.NormalizeCapabilityConnectorType(manifest.Spec.Type) {
	case "mcp":
		return "mcp"
	case "skill":
		return "skill"
	default:
		return "connector"
	}
}

func capabilityRuntimeKey(manifest connectors.Manifest) string {
	normalizedType := connectors.NormalizeCapabilityConnectorType(manifest.Spec.Type)
	switch normalizedType {
	case "mcp", "skill":
		return normalizedType
	}
	protocol := strings.ToLower(strings.TrimSpace(manifest.Spec.Protocol))
	if protocol != "" {
		return protocol
	}
	return normalizedType
}

func cloneCapabilityRuntimes(input map[string]CapabilityRuntime) map[string]CapabilityRuntime {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]CapabilityRuntime, len(input))
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || value == nil {
			continue
		}
		out[trimmed] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
