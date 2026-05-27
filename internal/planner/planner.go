package planner

import (
	"fmt"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

const (
	actionCreateOrMerge = "create-or-merge"
	actionManual        = "manual"
	actionSkip          = "skip"
	actionRedact        = "redact"
)

// Build converts a read-only scan report into a read-only migration plan.
func Build(scan model.ScanReport) model.PlanReport {
	plan := model.PlanReport{
		SchemaVersion: model.PlanSchemaVersion,
		Source:        scan.Source,
		Target:        scan.Target,
		Project:       scan.Project,
		Operations:    make([]model.Operation, 0, len(scan.Resources)),
		Warnings:      append([]model.Warning(nil), scan.Warnings...),
		NonGoals:      nonGoals(),
	}

	for index, resource := range scan.Resources {
		operation := model.Operation{
			ID:         fmt.Sprintf("op-%03d", index+1),
			ResourceID: resource.ID,
			Kind:       resource.Kind,
			SourcePath: resource.SourcePath,
			TargetPath: resource.TargetPathHint,
			Status:     resource.Status,
			Strategy:   resource.Strategy,
			Warnings:   append([]model.Warning(nil), resource.Warnings...),
		}

		operation.Action, operation.RequiresReview = actionFor(resource)
		plan.Operations = append(plan.Operations, operation)
		countOperation(&plan.Summary, operation)
	}

	return plan
}

func actionFor(resource model.Resource) (string, bool) {
	switch resource.Status {
	case model.StatusCompatible:
		return actionCreateOrMerge, false
	case model.StatusPartial:
		return actionManual, true
	case model.StatusUnsupported:
		return actionSkip, true
	case model.StatusDangerous:
		if hasWarningCode(resource.Warnings, "secret-redacted") {
			return actionRedact, true
		}
		return actionManual, true
	default:
		return actionManual, true
	}
}

func countOperation(summary *model.PlanSummary, operation model.Operation) {
	switch operation.Action {
	case actionCreateOrMerge:
		summary.Modify++
	case actionSkip:
		summary.Skip++
	case actionManual:
		summary.Manual++
	}

	if operation.Status == model.StatusDangerous {
		summary.Dangerous++
	}
}

func hasWarningCode(warnings []model.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func nonGoals() []string {
	return []string{
		"export",
		"apply",
		"sync",
		"conflicts/resolve",
		"three-way merge",
		"real home writes",
		"complete memory migration",
		"historical session migration",
		"hook execution",
		"model invocation",
		"plugin/skill/MCP installation",
		"automatic git push/commit/config",
	}
}
