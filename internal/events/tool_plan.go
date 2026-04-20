package events

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"
	"time"

	"tars/internal/contracts"
	"tars/internal/foundation/audit"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type executedToolContext struct {
	Context      map[string]interface{}
	Attachments  []contracts.MessageAttachment
	ExecutedPlan []contracts.ToolPlanStep
}

func (d *Dispatcher) executeToolPlan(ctx context.Context, sessionDetail contracts.SessionDetail, steps []contracts.ToolPlanStep) (executedToolContext, error) {
	result := executedToolContext{Context: map[string]interface{}{}}
	if len(steps) == 0 {
		return result, nil
	}

	toolResults := make([]map[string]interface{}, 0, len(steps))
	allAttachments := make([]contracts.MessageAttachment, 0, 2)
	executedPlan := make([]contracts.ToolPlanStep, 0, len(steps))

	for _, original := range steps {
		step := original
		step.Input = cloneMap(step.Input)
		step.ResolvedInput = resolveToolPlanInput(step.Input, sessionDetail, result.Context, executedPlan)
		if strings.TrimSpace(step.Status) == "" {
			step.Status = "planned"
		}

		switch strings.TrimSpace(step.Tool) {
		case "metrics.query_range", "metrics.query_instant":
			step.StartedAt = time.Now().UTC()
			step.Status = "running"
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_started", step, nil)

			if strings.TrimSpace(step.ConnectorID) == "" {
				step.ConnectorID = stringValue(step.ResolvedInput, "connector_id", "")
			}
			if strings.TrimSpace(step.ConnectorID) == "" {
				step.ConnectorID = d.resolveConnectorIDByType("metrics")
			}
			if strings.TrimSpace(step.ConnectorID) != "" {
				if step.ResolvedInput == nil {
					step.ResolvedInput = map[string]interface{}{}
				}
				step.ResolvedInput["connector_id"] = step.ConnectorID
			}

			metricsQuery := contracts.MetricsQuery{
				Service:         stringValue(step.ResolvedInput, "service", alertLabel(sessionDetail.Alert, "service")),
				Host:            stringValue(step.ResolvedInput, "host", stringFromAlert(sessionDetail.Alert, "host")),
				Query:           stringValue(step.ResolvedInput, "query", buildDiagnosisMetricsQuery(stringValue(step.ResolvedInput, "host", stringFromAlert(sessionDetail.Alert, "host")), stringValue(step.ResolvedInput, "service", alertLabel(sessionDetail.Alert, "service")))),
				Mode:            stringValue(step.ResolvedInput, "mode", defaultMetricsMode(step.Tool)),
				Step:            stringValue(step.ResolvedInput, "step", "5m"),
				Window:          stringValue(step.ResolvedInput, "window", "1h"),
				ConnectorID:     strings.TrimSpace(step.ConnectorID),
				ConnectorType:   stringValue(step.ResolvedInput, "connector_type", ""),
				ConnectorVendor: stringValue(step.ResolvedInput, "connector_vendor", ""),
				Protocol:        stringValue(step.ResolvedInput, "protocol", ""),
			}
			if metricsQuery.Mode == "range" {
				metricsQuery.Mode = "range"
			}

			metricsResult, err := d.action.QueryMetrics(ctx, metricsQuery)
			step.CompletedAt = time.Now().UTC()
			if err != nil {
				step.Status = "failed"
				step.Output = map[string]interface{}{
					"error": err.Error(),
				}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": err.Error()})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"error":          err.Error(),
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
				})
				if shouldStopToolPlan(step.OnFailure) {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}

			step.Status = "completed"
			step.Runtime = contracts.CloneRuntimeMetadata(metricsResult.Runtime)
			if strings.TrimSpace(step.ConnectorID) == "" && metricsResult.Runtime != nil {
				step.ConnectorID = strings.TrimSpace(metricsResult.Runtime.ConnectorID)
			}
			step.Output = map[string]interface{}{
				"series_count": len(metricsResult.Series),
				"points":       metricsPointCount(metricsResult.Series),
				"query":        metricsQuery.Query,
				"mode":         metricsQuery.Mode,
				"window":       metricsQuery.Window,
				"step":         metricsQuery.Step,
			}
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_completed", step, map[string]any{
				"series_count": len(metricsResult.Series),
				"points":       metricsPointCount(metricsResult.Series),
			})

			result.Context["metrics_series"] = metricsResult.Series
			result.Context["metrics_query"] = metricsQuery.Query
			result.Context["metrics_query_mode"] = metricsQuery.Mode
			result.Context["metrics_query_window"] = metricsQuery.Window
			result.Context["metrics_query_step"] = metricsQuery.Step
			result.Context["metrics_runtime"] = runtimeMap(metricsResult.Runtime)

			attachments := buildMetricsAttachments(metricsQuery, metricsResult)
			if analysis, attachment := buildDiskMetricsAnalysis(sessionDetail, step, metricsQuery, metricsResult); len(analysis) > 0 {
				step.Output["analysis"] = cloneMap(analysis)
				result.Context["disk_space_analysis"] = mergeInterfaceMaps(interfaceMap(result.Context["disk_space_analysis"]), analysis)
				if attachment != nil {
					attachments = append(attachments, *attachment)
				}
			}
			allAttachments = append(allAttachments, attachments...)
			executedPlan = append(executedPlan, step)
			toolResults = append(toolResults, map[string]interface{}{
				"tool":           step.Tool,
				"status":         step.Status,
				"input":          cloneMap(step.Input),
				"resolved_input": cloneMap(step.ResolvedInput),
				"output":         cloneMap(step.Output),
				"runtime":        runtimeMap(step.Runtime),
				"attachments":    len(attachments),
			})
		case "knowledge.search":
			step.StartedAt = time.Now().UTC()
			step.Status = "running"
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_started", step, nil)

			query := stringValue(step.ResolvedInput, "query", fallbackString(annotationString(sessionDetail.Alert, "user_request"), annotationString(sessionDetail.Alert, "summary")))
			hits, err := d.knowledge.Search(ctx, contracts.KnowledgeQuery{Query: query})
			step.CompletedAt = time.Now().UTC()
			if err != nil {
				step.Status = "failed"
				step.Output = map[string]interface{}{"error": err.Error()}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": err.Error()})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"error":          err.Error(),
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
				})
				if shouldStopToolPlan(step.OnFailure) {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}

			step.Status = "completed"
			step.Output = map[string]interface{}{
				"hit_count": len(hits),
			}
			result.Context["knowledge_hits"] = hits
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_completed", step, map[string]any{"hit_count": len(hits)})
			executedPlan = append(executedPlan, step)
			toolResults = append(toolResults, map[string]interface{}{
				"tool":           step.Tool,
				"status":         step.Status,
				"input":          cloneMap(step.Input),
				"resolved_input": cloneMap(step.ResolvedInput),
				"output":         cloneMap(step.Output),
				"hit_count":      len(hits),
			})
		case "connector.invoke_capability":
			step.StartedAt = time.Now().UTC()
			step.Status = "running"
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_started", step, nil)

			connID := strings.TrimSpace(step.ConnectorID)
			capID := stringValue(step.ResolvedInput, "capability_id", "")
			if connID == "" {
				connID = stringValue(step.ResolvedInput, "connector_id", "")
			}
			backfillStepConnector(&step, connID)
			if capID == "" {
				step.CompletedAt = time.Now().UTC()
				step.Status = "failed"
				step.Output = map[string]interface{}{"error": "capability_id is required in step input"}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": "capability_id is required"})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"error":          "capability_id is required",
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
				})
				if shouldStopToolPlan(step.OnFailure) {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}

			capResult, err := d.action.InvokeCapability(ctx, contracts.CapabilityRequest{
				ConnectorID:  connID,
				CapabilityID: capID,
				Params:       cloneMap(step.ResolvedInput),
				SessionID:    sessionDetail.SessionID,
				Caller:       "tool_plan_executor",
			})
			step.CompletedAt = time.Now().UTC()
			if err != nil {
				step.Status = "failed"
				step.Output = map[string]interface{}{
					"error":  err.Error(),
					"status": capResult.Status,
				}
				if capResult.Runtime != nil {
					step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
				}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": err.Error(), "capability_status": capResult.Status})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"error":          err.Error(),
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
				})
				if shouldStopToolPlan(step.OnFailure) {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}
			if capResult.Status == "pending_approval" || capResult.Status == "denied" {
				step.Status = capResult.Status
				step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
				if strings.TrimSpace(step.ConnectorID) == "" && capResult.Runtime != nil {
					step.ConnectorID = strings.TrimSpace(capResult.Runtime.ConnectorID)
				}
				step.Output = map[string]interface{}{
					"status":        capResult.Status,
					"capability_id": capID,
					"connector_id":  connID,
					"error":         capResult.Error,
				}
				if len(capResult.Metadata) > 0 {
					step.Output["metadata"] = cloneMap(capResult.Metadata)
				}
				if capResult.Status == "pending_approval" && d.workflow != nil {
					approvalDetail, messages, approvalErr := d.workflow.CreateCapabilityApproval(ctx, contracts.ApprovedCapabilityRequest{
						SessionID:    sessionDetail.SessionID,
						StepID:       step.ID,
						ConnectorID:  connID,
						CapabilityID: capID,
						Params:       cloneMap(step.ResolvedInput),
						RequestedBy:  "tool_plan_executor",
						Runtime:      contracts.CloneRuntimeMetadata(step.Runtime),
					})
					if approvalErr != nil {
						step.Status = "failed"
						step.Output["error"] = approvalErr.Error()
						d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": approvalErr.Error()})
					} else {
						step.Output["approval_id"] = approvalDetail.ExecutionID
						step.Output["notification_count"] = len(messages)
						if deliveryErr := deliverToolPlanNotifications(ctx, d.channel, d.workflow, sessionDetail.SessionID, messages); deliveryErr != nil {
							step.Status = "failed"
							step.Output["error"] = deliveryErr.Error()
							d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": deliveryErr.Error()})
						}
					}
				}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_"+capResult.Status, step, map[string]any{
					"capability_id":     capID,
					"capability_status": capResult.Status,
				})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
					"output":         cloneMap(step.Output),
					"runtime":        runtimeMap(step.Runtime),
				})
				if (capResult.Status == "pending_approval" && shouldStopToolPlan(step.OnPendingApproval)) || (capResult.Status == "denied" && shouldStopToolPlan(step.OnDenied)) || step.Status == "failed" {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}

			step.Status = "completed"
			step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
			if strings.TrimSpace(step.ConnectorID) == "" && capResult.Runtime != nil {
				step.ConnectorID = strings.TrimSpace(capResult.Runtime.ConnectorID)
			}
			step.Output = map[string]interface{}{
				"status":        capResult.Status,
				"capability_id": capID,
				"connector_id":  connID,
			}
			if len(capResult.Output) > 0 {
				step.Output["result"] = capResult.Output
			}
			if len(capResult.Artifacts) > 0 {
				step.Output["artifact_count"] = len(capResult.Artifacts)
				allAttachments = append(allAttachments, capResult.Artifacts...)
			}
			result.Context["capability_result_"+capID] = capResult.Output
			if len(capResult.Metadata) > 0 {
				result.Context["capability_metadata_"+capID] = capResult.Metadata
			}
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_completed", step, map[string]any{
				"capability_id":     capID,
				"capability_status": capResult.Status,
			})
			executedPlan = append(executedPlan, step)
			toolResults = append(toolResults, map[string]interface{}{
				"tool":           step.Tool,
				"status":         step.Status,
				"input":          cloneMap(step.Input),
				"resolved_input": cloneMap(step.ResolvedInput),
				"output":         cloneMap(step.Output),
				"runtime":        runtimeMap(step.Runtime),
			})
		case "logs.query", "observability.query":
			step.StartedAt = time.Now().UTC()
			step.Status = "running"
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_started", step, nil)

			obsConnID := strings.TrimSpace(step.ConnectorID)
			if obsConnID == "" {
				obsConnID = stringValue(step.ResolvedInput, "connector_id", "")
			}
			if obsConnID == "" {
				if step.Tool == "logs.query" {
					obsConnID = d.resolveConnectorIDByType("logs")
				} else {
					obsConnID = d.resolveConnectorIDByType("observability")
				}
			}
			defaultCapID := "query"
			if step.Tool == "logs.query" {
				defaultCapID = "logs.query"
			}
			obsCapID := stringValue(step.ResolvedInput, "capability_id", defaultCapID)
			backfillStepConnector(&step, obsConnID)

			capResult, err := d.action.InvokeCapability(ctx, contracts.CapabilityRequest{
				ConnectorID:  obsConnID,
				CapabilityID: obsCapID,
				Params:       cloneMap(step.ResolvedInput),
				SessionID:    sessionDetail.SessionID,
				Caller:       "tool_plan_executor",
			})
			step.CompletedAt = time.Now().UTC()
			if err != nil {
				step.Status = "failed"
				step.Output = map[string]interface{}{
					"error":  err.Error(),
					"status": capResult.Status,
				}
				if capResult.Runtime != nil {
					step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
				}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": err.Error()})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"error":          err.Error(),
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
				})
				if shouldStopToolPlan(step.OnFailure) {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}
			if capResult.Status == "pending_approval" || capResult.Status == "denied" {
				step.Status = capResult.Status
				step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
				if strings.TrimSpace(step.ConnectorID) == "" && capResult.Runtime != nil {
					step.ConnectorID = strings.TrimSpace(capResult.Runtime.ConnectorID)
				}
				step.Output = map[string]interface{}{
					"status":       capResult.Status,
					"connector_id": obsConnID,
					"error":        capResult.Error,
				}
				if len(capResult.Metadata) > 0 {
					step.Output["metadata"] = cloneMap(capResult.Metadata)
				}
				if capResult.Status == "pending_approval" && d.workflow != nil {
					approvalDetail, messages, approvalErr := d.workflow.CreateCapabilityApproval(ctx, contracts.ApprovedCapabilityRequest{
						SessionID:    sessionDetail.SessionID,
						StepID:       step.ID,
						ConnectorID:  obsConnID,
						CapabilityID: obsCapID,
						Params:       cloneMap(step.ResolvedInput),
						RequestedBy:  "tool_plan_executor",
						Runtime:      contracts.CloneRuntimeMetadata(step.Runtime),
					})
					if approvalErr != nil {
						step.Status = "failed"
						step.Output["error"] = approvalErr.Error()
						d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": approvalErr.Error()})
					} else {
						step.Output["approval_id"] = approvalDetail.ExecutionID
						step.Output["notification_count"] = len(messages)
						if deliveryErr := deliverToolPlanNotifications(ctx, d.channel, d.workflow, sessionDetail.SessionID, messages); deliveryErr != nil {
							step.Status = "failed"
							step.Output["error"] = deliveryErr.Error()
							d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": deliveryErr.Error()})
						}
					}
				}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_"+capResult.Status, step, map[string]any{
					"connector_id":      obsConnID,
					"capability_status": capResult.Status,
				})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
					"output":         cloneMap(step.Output),
					"runtime":        runtimeMap(step.Runtime),
				})
				if (capResult.Status == "pending_approval" && shouldStopToolPlan(step.OnPendingApproval)) || (capResult.Status == "denied" && shouldStopToolPlan(step.OnDenied)) || step.Status == "failed" {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}

			step.Status = "completed"
			step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
			if strings.TrimSpace(step.ConnectorID) == "" && capResult.Runtime != nil {
				step.ConnectorID = strings.TrimSpace(capResult.Runtime.ConnectorID)
			}
			step.Output = map[string]interface{}{
				"status":       capResult.Status,
				"connector_id": obsConnID,
			}
			if len(capResult.Output) > 0 {
				step.Output["result"] = capResult.Output
			}
			if len(capResult.Artifacts) > 0 {
				step.Output["artifact_count"] = len(capResult.Artifacts)
				allAttachments = append(allAttachments, capResult.Artifacts...)
			}
			if step.Tool == "logs.query" {
				result.Context["logs_query_result"] = capResult.Output
			} else {
				result.Context["observability_query_result"] = capResult.Output
			}
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_completed", step, map[string]any{
				"connector_id":      obsConnID,
				"capability_status": capResult.Status,
			})
			executedPlan = append(executedPlan, step)
			toolResults = append(toolResults, map[string]interface{}{
				"tool":           step.Tool,
				"status":         step.Status,
				"input":          cloneMap(step.Input),
				"resolved_input": cloneMap(step.ResolvedInput),
				"output":         cloneMap(step.Output),
				"runtime":        runtimeMap(step.Runtime),
			})
		case "delivery.query":
			step.StartedAt = time.Now().UTC()
			step.Status = "running"
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_started", step, nil)

			delConnID := strings.TrimSpace(step.ConnectorID)
			if delConnID == "" {
				delConnID = stringValue(step.ResolvedInput, "connector_id", "")
			}
			if delConnID == "" {
				delConnID = d.resolveConnectorIDByType("delivery")
			}
			delCapID := stringValue(step.ResolvedInput, "capability_id", "query")
			backfillStepConnector(&step, delConnID)

			capResult, err := d.action.InvokeCapability(ctx, contracts.CapabilityRequest{
				ConnectorID:  delConnID,
				CapabilityID: delCapID,
				Params:       cloneMap(step.ResolvedInput),
				SessionID:    sessionDetail.SessionID,
				Caller:       "tool_plan_executor",
			})
			step.CompletedAt = time.Now().UTC()
			if err != nil {
				step.Status = "failed"
				step.Output = map[string]interface{}{
					"error":  err.Error(),
					"status": capResult.Status,
				}
				if capResult.Runtime != nil {
					step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
				}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": err.Error()})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"error":          err.Error(),
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
				})
				if shouldStopToolPlan(step.OnFailure) {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}
			if capResult.Status == "pending_approval" || capResult.Status == "denied" {
				step.Status = capResult.Status
				step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
				if strings.TrimSpace(step.ConnectorID) == "" && capResult.Runtime != nil {
					step.ConnectorID = strings.TrimSpace(capResult.Runtime.ConnectorID)
				}
				step.Output = map[string]interface{}{
					"status":       capResult.Status,
					"connector_id": delConnID,
					"error":        capResult.Error,
				}
				if len(capResult.Metadata) > 0 {
					step.Output["metadata"] = cloneMap(capResult.Metadata)
				}
				if capResult.Status == "pending_approval" && d.workflow != nil {
					approvalDetail, messages, approvalErr := d.workflow.CreateCapabilityApproval(ctx, contracts.ApprovedCapabilityRequest{
						SessionID:    sessionDetail.SessionID,
						StepID:       step.ID,
						ConnectorID:  delConnID,
						CapabilityID: delCapID,
						Params:       cloneMap(step.ResolvedInput),
						RequestedBy:  "tool_plan_executor",
						Runtime:      contracts.CloneRuntimeMetadata(step.Runtime),
					})
					if approvalErr != nil {
						step.Status = "failed"
						step.Output["error"] = approvalErr.Error()
						d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": approvalErr.Error()})
					} else {
						step.Output["approval_id"] = approvalDetail.ExecutionID
						step.Output["notification_count"] = len(messages)
						if deliveryErr := deliverToolPlanNotifications(ctx, d.channel, d.workflow, sessionDetail.SessionID, messages); deliveryErr != nil {
							step.Status = "failed"
							step.Output["error"] = deliveryErr.Error()
							d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_failed", step, map[string]any{"error": deliveryErr.Error()})
						}
					}
				}
				d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_"+capResult.Status, step, map[string]any{
					"connector_id":      delConnID,
					"capability_status": capResult.Status,
				})
				executedPlan = append(executedPlan, step)
				toolResults = append(toolResults, map[string]interface{}{
					"tool":           step.Tool,
					"status":         step.Status,
					"input":          cloneMap(step.Input),
					"resolved_input": cloneMap(step.ResolvedInput),
					"output":         cloneMap(step.Output),
					"runtime":        runtimeMap(step.Runtime),
				})
				if (capResult.Status == "pending_approval" && shouldStopToolPlan(step.OnPendingApproval)) || (capResult.Status == "denied" && shouldStopToolPlan(step.OnDenied)) || step.Status == "failed" {
					result.Context["tool_results"] = toolResults
					result.Attachments = allAttachments
					result.ExecutedPlan = executedPlan
					return result, nil
				}
				continue
			}

			step.Status = "completed"
			step.Runtime = contracts.CloneRuntimeMetadata(capResult.Runtime)
			if strings.TrimSpace(step.ConnectorID) == "" && capResult.Runtime != nil {
				step.ConnectorID = strings.TrimSpace(capResult.Runtime.ConnectorID)
			}
			step.Output = map[string]interface{}{
				"status":       capResult.Status,
				"connector_id": delConnID,
			}
			if len(capResult.Output) > 0 {
				step.Output["result"] = capResult.Output
			}
			if len(capResult.Artifacts) > 0 {
				step.Output["artifact_count"] = len(capResult.Artifacts)
				allAttachments = append(allAttachments, capResult.Artifacts...)
			}
			result.Context["delivery_query_result"] = capResult.Output
			d.auditToolPlanStep(ctx, sessionDetail.SessionID, "tool_plan_step_completed", step, map[string]any{
				"connector_id":      delConnID,
				"capability_status": capResult.Status,
			})
			executedPlan = append(executedPlan, step)
			toolResults = append(toolResults, map[string]interface{}{
				"tool":           step.Tool,
				"status":         step.Status,
				"input":          cloneMap(step.Input),
				"resolved_input": cloneMap(step.ResolvedInput),
				"output":         cloneMap(step.Output),
				"runtime":        runtimeMap(step.Runtime),
			})
		default:
			executedPlan = append(executedPlan, step)
			toolResults = append(toolResults, map[string]interface{}{
				"tool":   step.Tool,
				"status": step.Status,
				"input":  cloneMap(step.Input),
			})
		}
	}

	result.Context["tool_results"] = toolResults
	result.Attachments = allAttachments
	result.ExecutedPlan = executedPlan
	return result, nil
}

func deliverToolPlanNotifications(ctx context.Context, channel contracts.ChannelService, workflow contracts.WorkflowService, sessionID string, messages []contracts.ChannelMessage) error {
	for _, message := range messages {
		if channel == nil {
			if workflow == nil || strings.TrimSpace(sessionID) == "" {
				return fmt.Errorf("channel service unavailable")
			}
			if enqueueErr := workflow.EnqueueNotifications(ctx, sessionID, []contracts.ChannelMessage{message}); enqueueErr != nil {
				return enqueueErr
			}
			continue
		}
		if _, err := channel.SendMessage(ctx, message); err != nil {
			if workflow == nil || strings.TrimSpace(sessionID) == "" {
				return err
			}
			if enqueueErr := workflow.EnqueueNotifications(ctx, sessionID, []contracts.ChannelMessage{message}); enqueueErr != nil {
				return enqueueErr
			}
		}
	}
	return nil
}

func backfillStepConnector(step *contracts.ToolPlanStep, connectorID string) {
	if step == nil {
		return
	}
	connectorID = strings.TrimSpace(connectorID)
	if connectorID == "" {
		return
	}
	if strings.TrimSpace(step.ConnectorID) == "" {
		step.ConnectorID = connectorID
	}
	if step.ResolvedInput == nil {
		step.ResolvedInput = map[string]interface{}{}
	}
	if strings.TrimSpace(stringValue(step.ResolvedInput, "connector_id", "")) == "" {
		step.ResolvedInput["connector_id"] = connectorID
	}
}

func (d *Dispatcher) auditToolPlanGenerated(ctx context.Context, sessionID string, plan contracts.DiagnosisPlan) {
	if d == nil || d.audit == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	items := make([]map[string]any, 0, len(plan.ToolPlan))
	for _, step := range plan.ToolPlan {
		items = append(items, map[string]any{
			"tool":         step.Tool,
			"connector_id": step.ConnectorID,
			"reason":       step.Reason,
			"priority":     step.Priority,
			"status":       step.Status,
		})
	}
	d.audit.Log(ctx, audit.Entry{
		ResourceType: "tool_plan",
		ResourceID:   sessionID,
		Action:       "tool_plan_generated",
		Actor:        "tars_dispatcher",
		Metadata: map[string]any{
			"session_id": sessionID,
			"summary":    plan.Summary,
			"step_count": len(plan.ToolPlan),
			"steps":      items,
		},
	})
}

func (d *Dispatcher) auditToolPlanStep(ctx context.Context, sessionID string, action string, step contracts.ToolPlanStep, extra map[string]any) {
	if d == nil || d.audit == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	metadata := map[string]any{
		"session_id":   sessionID,
		"tool":         step.Tool,
		"connector_id": step.ConnectorID,
		"reason":       step.Reason,
		"status":       step.Status,
		"priority":     step.Priority,
		"input":        cloneMap(step.Input),
		"output":       cloneMap(step.Output),
		"started_at":   step.StartedAt,
		"completed_at": step.CompletedAt,
		"runtime":      runtimeMap(step.Runtime),
	}
	for key, value := range extra {
		metadata[key] = value
	}
	d.audit.Log(ctx, audit.Entry{
		ResourceType: "tool_plan",
		ResourceID:   sessionID,
		Action:       action,
		Actor:        "tars_dispatcher",
		Metadata:     metadata,
	})
}

func buildMetricsAttachments(query contracts.MetricsQuery, result contracts.MetricsResult) []contracts.MessageAttachment {
	if len(result.Series) == 0 {
		return nil
	}

	attachments := make([]contracts.MessageAttachment, 0, 2)
	prettyJSON, err := json.MarshalIndent(result.Series, "", "  ")
	if err == nil {
		attachments = append(attachments, contracts.MessageAttachment{
			Type:        "file",
			Name:        fmt.Sprintf("metrics-%s.json", strings.TrimSpace(firstNonEmpty(query.Mode, "query"))),
			MimeType:    "application/json",
			Content:     string(prettyJSON),
			PreviewText: fmt.Sprintf("Metrics query result (%d series, %d points).", len(result.Series), metricsPointCount(result.Series)),
			Metadata: map[string]interface{}{
				"source": "metrics",
				"mode":   firstNonEmpty(query.Mode, "instant"),
			},
		})
	}

	imageBytes, previewText, err := renderMetricsPNG(query, result.Series)
	if err == nil && len(imageBytes) > 0 {
		attachments = append(attachments, contracts.MessageAttachment{
			Type:        "image",
			Name:        fmt.Sprintf("metrics-%s.png", strings.TrimSpace(firstNonEmpty(query.Mode, "query"))),
			MimeType:    "image/png",
			Content:     base64.StdEncoding.EncodeToString(imageBytes),
			PreviewText: previewText,
			Metadata: map[string]interface{}{
				"encoding": "base64",
				"source":   "metrics",
				"mode":     firstNonEmpty(query.Mode, "instant"),
			},
		})
	}

	return attachments
}

func renderMetricsPNG(query contracts.MetricsQuery, series []map[string]interface{}) ([]byte, string, error) {
	points := flattenMetricPoints(series)
	if len(points) == 0 {
		return nil, "", fmt.Errorf("no time-series points available")
	}

	const (
		width         = 960
		height        = 420
		leftPadding   = 84
		rightPadding  = 28
		topPadding    = 56
		bottomPadding = 62
	)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bg := color.RGBA{R: 18, G: 22, B: 31, A: 255}
	grid := color.RGBA{R: 60, G: 70, B: 90, A: 255}
	line := color.RGBA{R: 70, G: 168, B: 255, A: 255}
	axis := color.RGBA{R: 120, G: 130, B: 150, A: 255}
	text := color.RGBA{R: 224, G: 229, B: 236, A: 255}
	muted := color.RGBA{R: 155, G: 166, B: 181, A: 255}
	fillImage(img, bg)
	for i := 0; i < 4; i++ {
		y := topPadding + i*(height-topPadding-bottomPadding)/3
		drawLine(img, leftPadding, y, width-rightPadding, y, grid)
	}
	drawLine(img, leftPadding, topPadding, leftPadding, height-bottomPadding, axis)
	drawLine(img, leftPadding, height-bottomPadding, width-rightPadding, height-bottomPadding, axis)

	minVal := points[0].value
	maxVal := points[0].value
	for _, point := range points[1:] {
		if point.value < minVal {
			minVal = point.value
		}
		if point.value > maxVal {
			maxVal = point.value
		}
	}
	if minVal == maxVal {
		minVal -= 1
		maxVal += 1
	}

	xStep := float64(width-leftPadding-rightPadding) / math.Max(float64(len(points)-1), 1)
	for idx := 1; idx < len(points); idx++ {
		prevX := leftPadding + int(float64(idx-1)*xStep)
		nextX := leftPadding + int(float64(idx)*xStep)
		prevY := scaleMetricPoint(points[idx-1].value, minVal, maxVal, height, topPadding, bottomPadding)
		nextY := scaleMetricPoint(points[idx].value, minVal, maxVal, height, topPadding, bottomPadding)
		drawLine(img, prevX, prevY, nextX, nextY, line)
	}

	title := metricsChartTitle(query, series)
	drawText(img, leftPadding, 26, title, text)
	drawText(img, leftPadding, 44, fmt.Sprintf("Window: %s  Step: %s", firstNonEmpty(query.Window, "instant"), firstNonEmpty(query.Step, "n/a")), muted)
	drawText(img, leftPadding, 60, trimText(query.Query, 96), muted)

	midVal := minVal + (maxVal-minVal)/2
	drawText(img, 10, topPadding+4, formatMetricValue(maxVal), muted)
	drawText(img, 10, topPadding+(height-topPadding-bottomPadding)/2+4, formatMetricValue(midVal), muted)
	drawText(img, 10, height-bottomPadding+4, formatMetricValue(minVal), muted)
	drawText(img, leftPadding, height-18, formatMetricTime(points[0].timestamp), muted)
	drawText(img, width-rightPadding-84, height-18, formatMetricTime(points[len(points)-1].timestamp), muted)
	drawText(img, width/2-14, height-18, "time", text)
	drawText(img, 10, 18, "value", text)

	buf := bytes.Buffer{}
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", err
	}

	preview := fmt.Sprintf("%s (%d points, min=%s max=%s).", title, len(points), formatMetricValue(minVal), formatMetricValue(maxVal))
	return buf.Bytes(), preview, nil
}

type metricPoint struct {
	timestamp time.Time
	value     float64
}

func flattenMetricPoints(series []map[string]interface{}) []metricPoint {
	points := make([]metricPoint, 0, 64)
	for _, row := range series {
		if values, ok := row["values"].([][]interface{}); ok {
			for _, pair := range values {
				if len(pair) != 2 {
					continue
				}
				timestamp, _ := interfaceToUnixTime(pair[0])
				value, ok := interfaceToFloat64(pair[1])
				if !ok {
					continue
				}
				points = append(points, metricPoint{timestamp: timestamp, value: value})
			}
			continue
		}
		if values, ok := row["values"].([]interface{}); ok {
			for _, item := range values {
				pair, ok := item.([]interface{})
				if !ok || len(pair) != 2 {
					continue
				}
				timestamp, _ := interfaceToUnixTime(pair[0])
				value, ok := interfaceToFloat64(pair[1])
				if !ok {
					continue
				}
				points = append(points, metricPoint{timestamp: timestamp, value: value})
			}
			continue
		}
		timestamp, _ := interfaceToUnixTime(row["timestamp"])
		if value, ok := interfaceToFloat64(row["value"]); ok {
			points = append(points, metricPoint{timestamp: timestamp, value: value})
		}
	}
	return points
}

func interfaceToUnixTime(value interface{}) (time.Time, bool) {
	switch typed := value.(type) {
	case float64:
		return time.Unix(int64(typed), 0).UTC(), true
	case float32:
		return time.Unix(int64(typed), 0).UTC(), true
	case int:
		return time.Unix(int64(typed), 0).UTC(), true
	case int64:
		return time.Unix(typed, 0).UTC(), true
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return time.Unix(int64(parsed), 0).UTC(), true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return time.Time{}, false
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return time.Unix(int64(parsed), 0).UTC(), true
		}
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func interfaceToFloat64(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		v, err := typed.Float64()
		return v, err == nil
	case string:
		v, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return v, err == nil
	default:
		return 0, false
	}
}

func scaleMetricPoint(value float64, minVal float64, maxVal float64, height int, topPadding int, bottomPadding int) int {
	ratio := (value - minVal) / (maxVal - minVal)
	return height - bottomPadding - int(ratio*float64(height-topPadding-bottomPadding))
}

func fillImage(img *image.RGBA, fill color.RGBA) {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetRGBA(x, y, fill)
		}
	}
}

func drawLine(img *image.RGBA, x0 int, y0 int, x1 int, y1 int, c color.RGBA) {
	dx := int(math.Abs(float64(x1 - x0)))
	dy := -int(math.Abs(float64(y1 - y0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	errVal := dx + dy
	for {
		if x0 >= 0 && x0 < img.Bounds().Dx() && y0 >= 0 && y0 < img.Bounds().Dy() {
			img.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		twiceErr := 2 * errVal
		if twiceErr >= dy {
			errVal += dy
			x0 += sx
		}
		if twiceErr <= dx {
			errVal += dx
			y0 += sy
		}
	}
}

func drawText(img *image.RGBA, x int, y int, text string, c color.RGBA) {
	if strings.TrimSpace(text) == "" {
		return
	}
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(text)
}

func metricsChartTitle(query contracts.MetricsQuery, series []map[string]interface{}) string {
	base := strings.TrimSpace(query.Query)
	if base == "" && len(series) > 0 {
		if metricName, ok := series[0]["__name__"].(string); ok && strings.TrimSpace(metricName) != "" {
			base = strings.TrimSpace(metricName)
		}
	}
	if base == "" {
		base = "Metrics trend"
	}
	if strings.EqualFold(strings.TrimSpace(query.Mode), "range") {
		return fmt.Sprintf("%s (%s)", trimText(base, 48), firstNonEmpty(query.Window, "range"))
	}
	return trimText(base, 56)
}

func trimText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func formatMetricValue(value float64) string {
	if math.Abs(value) >= 100 || math.Abs(value-math.Round(value)) < 0.001 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.2f", value)
}

func formatMetricTime(value time.Time) string {
	if value.IsZero() {
		return "n/a"
	}
	return value.Local().Format("15:04")
}

func metricsPointCount(series []map[string]interface{}) int {
	total := 0
	for _, row := range series {
		if values, ok := row["values"].([][]interface{}); ok {
			total += len(values)
			continue
		}
		if values, ok := row["values"].([]interface{}); ok {
			total += len(values)
			continue
		}
		if _, ok := row["value"]; ok {
			total++
		}
	}
	return total
}

func stringValue(input map[string]interface{}, key string, fallback string) string {
	if input == nil {
		return fallback
	}
	value, ok := input[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return fallback
		}
		return strings.TrimSpace(typed)
	default:
		return fallback
	}
}

func interfaceString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func runtimeMap(runtime *contracts.RuntimeMetadata) map[string]interface{} {
	if runtime == nil {
		return nil
	}
	return map[string]interface{}{
		"runtime":          runtime.Runtime,
		"selection":        runtime.Selection,
		"connector_id":     runtime.ConnectorID,
		"connector_type":   runtime.ConnectorType,
		"connector_vendor": runtime.ConnectorVendor,
		"protocol":         runtime.Protocol,
		"execution_mode":   runtime.ExecutionMode,
		"fallback_enabled": runtime.FallbackEnabled,
		"fallback_used":    runtime.FallbackUsed,
		"fallback_reason":  runtime.FallbackReason,
		"fallback_target":  runtime.FallbackTarget,
	}
}

func defaultMetricsMode(tool string) string {
	if strings.TrimSpace(tool) == "metrics.query_range" {
		return "range"
	}
	return "instant"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func alertLabel(alert map[string]interface{}, key string) string {
	labels, ok := alert["labels"].(map[string]string)
	if ok {
		return strings.TrimSpace(labels[key])
	}
	if rawLabels, ok := alert["labels"].(map[string]interface{}); ok {
		if value, ok := rawLabels[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringFromAlert(alert map[string]interface{}, key string) string {
	if value, ok := alert[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return alertLabel(alert, key)
}

func appendOrReplaceExecutionPlanStep(steps []contracts.ToolPlanStep, command string, host string, service string) []contracts.ToolPlanStep {
	command = strings.TrimSpace(command)
	if command == "" {
		return steps
	}
	for idx := range steps {
		if strings.TrimSpace(steps[idx].Tool) != "execution.run_command" {
			continue
		}
		if steps[idx].Input == nil {
			steps[idx].Input = map[string]interface{}{}
		}
		steps[idx].Status = "planned"
		steps[idx].Input["command"] = command
		if strings.TrimSpace(host) != "" {
			steps[idx].Input["host"] = host
		}
		if strings.TrimSpace(service) != "" {
			steps[idx].Input["service"] = service
		}
		return steps
	}
	return append(steps, contracts.ToolPlanStep{
		Tool:     "execution.run_command",
		Reason:   "Host-level command is still required after tool-based diagnosis.",
		Priority: len(steps) + 1,
		Status:   "planned",
		Input: map[string]interface{}{
			"command": command,
			"host":    host,
			"service": service,
		},
	})
}

func removeExecutionPlanSteps(steps []contracts.ToolPlanStep) []contracts.ToolPlanStep {
	if len(steps) == 0 {
		return steps
	}
	out := make([]contracts.ToolPlanStep, 0, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.Tool) == "execution.run_command" {
			continue
		}
		out = append(out, step)
	}
	return out
}

func (d *Dispatcher) resolveConnectorIDByType(connectorType string) string {
	if d == nil || d.connectors == nil {
		return ""
	}
	snapshot := d.connectors.Snapshot()
	connType := strings.ToLower(strings.TrimSpace(connectorType))
	for _, entry := range snapshot.Config.Entries {
		if !entry.Enabled() {
			continue
		}
		if strings.ToLower(strings.TrimSpace(entry.Spec.Type)) == connType {
			return strings.TrimSpace(entry.Metadata.ID)
		}
	}
	return ""
}

func mergeStringMaps(left map[string]string, right map[string]string) map[string]string {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := make(map[string]string, len(left)+len(right))
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func mergeInterfaceMaps(left map[string]interface{}, right map[string]interface{}) map[string]interface{} {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(left)+len(right))
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func interfaceMap(value interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return typed
	}
	return nil
}

func shouldStopToolPlan(policy string) bool {
	return strings.EqualFold(strings.TrimSpace(policy), "stop")
}

func buildDiskMetricsAnalysis(sessionDetail contracts.SessionDetail, step contracts.ToolPlanStep, query contracts.MetricsQuery, result contracts.MetricsResult) (map[string]interface{}, *contracts.MessageAttachment) {
	if !isDiskMetricsContext(sessionDetail, step, query) || len(result.Series) == 0 {
		return nil, nil
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(query.Query)), "predict_linear") || strings.Contains(strings.ToLower(strings.TrimSpace(step.ID)), "forecast") {
		return buildDiskForecastAnalysis(step, query, result.Series)
	}
	return buildDiskUsageAnalysis(step, query, result.Series)
}

func isDiskMetricsContext(sessionDetail contracts.SessionDetail, step contracts.ToolPlanStep, query contracts.MetricsQuery) bool {
	parts := []string{
		strings.ToLower(strings.TrimSpace(query.Query)),
		strings.ToLower(strings.TrimSpace(step.Reason)),
		strings.ToLower(strings.TrimSpace(step.ID)),
		strings.ToLower(strings.TrimSpace(annotationString(sessionDetail.Alert, "summary"))),
		strings.ToLower(strings.TrimSpace(annotationString(sessionDetail.Alert, "user_request"))),
		strings.ToLower(strings.TrimSpace(alertLabel(sessionDetail.Alert, "alertname"))),
	}
	for _, part := range parts {
		if strings.Contains(part, "filesystem") || strings.Contains(part, "disk") || strings.Contains(part, "storage") || strings.Contains(part, "磁盘") {
			return true
		}
	}
	return false
}

func buildDiskUsageAnalysis(step contracts.ToolPlanStep, query contracts.MetricsQuery, series []map[string]interface{}) (map[string]interface{}, *contracts.MessageAttachment) {
	type candidate struct {
		mountpoint string
		device     string
		current    float64
		start      float64
		peak       float64
		deltaHour  float64
	}
	var best candidate
	found := false
	for _, row := range series {
		points := metricPointsFromSeriesRow(row)
		if len(points) < 2 {
			continue
		}
		start := points[0].value
		current := points[len(points)-1].value
		peak := current
		for _, point := range points {
			if point.value > peak {
				peak = point.value
			}
		}
		hours := points[len(points)-1].timestamp.Sub(points[0].timestamp).Hours()
		if hours <= 0 {
			hours = 1
		}
		item := candidate{
			mountpoint: interfaceString(row["mountpoint"]),
			device:     interfaceString(row["device"]),
			current:    current,
			start:      start,
			peak:       peak,
			deltaHour:  (current - start) / hours,
		}
		if !found || item.current > best.current {
			best = item
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	analysis := map[string]interface{}{
		"step_id":                    step.ID,
		"analysis_type":              "usage_trend",
		"metric_query":               query.Query,
		"mountpoint":                 firstNonEmpty(best.mountpoint, "/"),
		"device":                     best.device,
		"current_usage_percent":      roundMetric(best.current),
		"starting_usage_percent":     roundMetric(best.start),
		"peak_usage_percent":         roundMetric(best.peak),
		"growth_percent_per_hour":    roundMetric(best.deltaHour),
		"summary":                    fmt.Sprintf("filesystem %s is at %s%% used, changing %s%% per hour", firstNonEmpty(best.mountpoint, "/"), formatMetricValue(best.current), formatMetricValue(best.deltaHour)),
		"tool_attachment_basename":   "disk-space-analysis",
		"tool_attachment_step_id":    step.ID,
		"tool_attachment_query_mode": query.Mode,
	}
	return analysis, buildDiskAnalysisAttachment(step.ID, analysis)
}

func buildDiskForecastAnalysis(step contracts.ToolPlanStep, query contracts.MetricsQuery, series []map[string]interface{}) (map[string]interface{}, *contracts.MessageAttachment) {
	type candidate struct {
		mountpoint string
		device     string
		predicted  float64
	}
	var best candidate
	found := false
	for _, row := range series {
		points := metricPointsFromSeriesRow(row)
		if len(points) == 0 {
			continue
		}
		predicted := points[len(points)-1].value
		item := candidate{
			mountpoint: interfaceString(row["mountpoint"]),
			device:     interfaceString(row["device"]),
			predicted:  predicted,
		}
		if !found || item.predicted < best.predicted {
			best = item
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	risk := "stable"
	if best.predicted <= 0 {
		risk = "fill_expected_within_forecast_window"
	} else if best.predicted < 10*1024*1024*1024 {
		risk = "low_free_space_within_forecast_window"
	}
	analysis := map[string]interface{}{
		"step_id":                        step.ID,
		"analysis_type":                  "capacity_forecast",
		"metric_query":                   query.Query,
		"mountpoint":                     firstNonEmpty(best.mountpoint, "/"),
		"device":                         best.device,
		"predicted_available_bytes":      roundMetric(best.predicted),
		"predicted_available_human":      formatBytes(best.predicted),
		"forecast_risk":                  risk,
		"forecast_window":                firstNonEmpty(query.Window, "1h"),
		"summary":                        fmt.Sprintf("forecast for %s leaves %s available; risk=%s", firstNonEmpty(best.mountpoint, "/"), formatBytes(best.predicted), risk),
		"tool_attachment_basename":       "disk-space-analysis",
		"tool_attachment_step_id":        step.ID,
		"tool_attachment_forecast_query": true,
	}
	return analysis, buildDiskAnalysisAttachment(step.ID, analysis)
}

func metricPointsFromSeriesRow(row map[string]interface{}) []metricPoint {
	if len(row) == 0 {
		return nil
	}
	return flattenMetricPoints([]map[string]interface{}{row})
}

func buildDiskAnalysisAttachment(stepID string, analysis map[string]interface{}) *contracts.MessageAttachment {
	if len(analysis) == 0 {
		return nil
	}
	content, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return nil
	}
	name := "disk-space-analysis.json"
	if strings.TrimSpace(stepID) != "" {
		name = fmt.Sprintf("disk-space-analysis-%s.json", strings.TrimSpace(stepID))
	}
	return &contracts.MessageAttachment{
		Type:        "file",
		Name:        name,
		MimeType:    "application/json",
		Content:     string(content),
		PreviewText: stringValue(analysis, "summary", "disk space analysis"),
		Metadata: map[string]interface{}{
			"source":  "disk_space_analysis",
			"step_id": strings.TrimSpace(stepID),
		},
	}
}

func roundMetric(value float64) float64 {
	return math.Round(value*100) / 100
}

func formatBytes(value float64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	idx := 0
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}
	formatted := fmt.Sprintf("%.1f %s", value, units[idx])
	if idx == 0 {
		formatted = fmt.Sprintf("%.0f %s", value, units[idx])
	}
	if negative {
		return "-" + formatted
	}
	return formatted
}

func resolveToolPlanInput(input map[string]interface{}, sessionDetail contracts.SessionDetail, context map[string]interface{}, executed []contracts.ToolPlanStep) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	resolved := make(map[string]interface{}, len(input))
	for key, value := range input {
		resolved[key] = resolveToolPlanValue(value, sessionDetail, context, executed)
	}
	return resolved
}

func resolveToolPlanValue(value interface{}, sessionDetail contracts.SessionDetail, context map[string]interface{}, executed []contracts.ToolPlanStep) interface{} {
	switch typed := value.(type) {
	case string:
		return resolveToolPlanString(typed, sessionDetail, context, executed)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			out[key] = resolveToolPlanValue(item, sessionDetail, context, executed)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, resolveToolPlanValue(item, sessionDetail, context, executed))
		}
		return out
	default:
		return value
	}
}

func resolveToolPlanString(value string, sessionDetail contracts.SessionDetail, context map[string]interface{}, executed []contracts.ToolPlanStep) interface{} {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "$") {
		return value
	}
	if strings.HasPrefix(trimmed, "$context.") {
		return lookupContextPath(context, strings.TrimPrefix(trimmed, "$context."))
	}
	if strings.HasPrefix(trimmed, "$alert.") {
		return lookupContextPath(sessionDetail.Alert, strings.TrimPrefix(trimmed, "$alert."))
	}
	if strings.HasPrefix(trimmed, "$steps.") {
		return lookupStepReference(trimmed, executed)
	}
	return value
}

func lookupStepReference(ref string, executed []contracts.ToolPlanStep) interface{} {
	parts := strings.Split(strings.TrimPrefix(ref, "$steps."), ".")
	if len(parts) == 0 {
		return nil
	}
	stepKey := strings.TrimSpace(parts[0])
	var step *contracts.ToolPlanStep
	for idx := range executed {
		candidate := &executed[idx]
		if strings.TrimSpace(candidate.ID) == stepKey {
			step = candidate
			break
		}
	}
	if step == nil {
		return nil
	}
	if len(parts) == 1 {
		return cloneMap(step.Output)
	}
	switch parts[1] {
	case "output":
		return lookupContextPath(step.Output, strings.Join(parts[2:], "."))
	case "input":
		return lookupContextPath(step.ResolvedInput, strings.Join(parts[2:], "."))
	case "status":
		return step.Status
	case "connector_id":
		return step.ConnectorID
	default:
		return nil
	}
}

func lookupContextPath(root map[string]interface{}, path string) interface{} {
	if len(root) == 0 {
		return nil
	}
	current := interface{}(root)
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}
