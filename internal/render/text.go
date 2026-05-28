package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/security"
)

func ScanText(writer io.Writer, report model.ScanReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon scan: %s -> %s", report.Source, report.Target); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Summary: compatible=%d partial=%d unsupported=%d dangerous=%d", report.Summary.Compatible, report.Summary.Partial, report.Summary.Unsupported, report.Summary.Dangerous); err != nil {
		return err
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Next steps:"); err != nil {
		return err
	}
	if err := out.line("- Run `agent-canon plan` to review proposed actions."); err != nil {
		return err
	}
	if err := out.line("- Review Partial, Unsupported, and Dangerous sections before applying changes."); err != nil {
		return err
	}
	groups := []struct {
		title       string
		status      model.Status
		description string
	}{
		{title: "Compatible", status: model.StatusCompatible, description: "These resources can be included in generated Codex previews."},
		{title: "Partial", status: model.StatusPartial, description: "These resources need review after generation because the target format is not equivalent."},
		{title: "Unsupported", status: model.StatusUnsupported, description: "These resources are skipped and need manual handling outside agent-canon."},
		{title: "Dangerous", status: model.StatusDangerous, description: "These resources contain sensitive or risky content and must be reviewed before any write."},
	}
	for _, group := range groups {
		if err := out.blank(); err != nil {
			return err
		}
		if err := out.line("%s:", group.title); err != nil {
			return err
		}
		if err := out.line("  %s", group.description); err != nil {
			return err
		}
		found := false
		for _, resource := range report.Resources {
			if resource.Status != group.status {
				continue
			}
			found = true
			if err := out.line("- %s [%s] %s", resource.ID, resource.Kind, resource.Strategy); err != nil {
				return err
			}
		}
		if !found {
			if err := out.line("- none"); err != nil {
				return err
			}
		}
	}
	return nil
}

func PlanText(writer io.Writer, report model.PlanReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon plan: %s -> %s", report.Source, report.Target); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Summary: modify=%d skip=%d manual=%d dangerous=%d", report.Summary.Modify, report.Summary.Skip, report.Summary.Manual, report.Summary.Dangerous); err != nil {
		return err
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Next steps:"); err != nil {
		return err
	}
	if err := out.line("- Run `agent-canon compile codex --out <dir>` to inspect generated files."); err != nil {
		return err
	}
	if err := out.line("- Run `agent-canon apply codex --dry-run` before any write."); err != nil {
		return err
	}

	actions := actionOrder(report.Operations)
	for _, action := range actions {
		if err := out.blank(); err != nil {
			return err
		}
		if err := out.line("%s:", action); err != nil {
			return err
		}
		if err := out.line("  %s", actionDescription(action)); err != nil {
			return err
		}
		for _, operation := range report.Operations {
			if operation.Action != action {
				continue
			}
			review := ""
			if operation.RequiresReview {
				review = " (Requires review)"
			}
			if err := out.line("- %s %s [%s]%s", operation.ID, operation.ResourceID, operation.Kind, review); err != nil {
				return err
			}
		}
	}

	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Requires review:"); err != nil {
		return err
	}
	found := false
	for _, operation := range report.Operations {
		if !operation.RequiresReview {
			continue
		}
		found = true
		if err := out.line("- %s %s [%s] action=%s strategy=%s status=%s", operation.ID, operation.ResourceID, operation.Kind, operation.Action, operation.Strategy, operation.Status); err != nil {
			return err
		}
	}
	if !found {
		return out.line("- none")
	}
	return nil
}

func InitText(writer io.Writer, report model.WorkspaceManifestReport, manifestPath string) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon init: %s -> %s", report.Source, report.Target); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Workspace: %s", report.WorkspaceRoot); err != nil {
		return err
	}
	if err := out.line("Manifest: %s", manifestPath); err != nil {
		return err
	}
	if len(report.Warnings) == 0 {
		return nil
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Warnings:"); err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		message, _ := security.RedactContent(warning.Message)
		if err := out.line("- warning[%s]: %s", warning.Code, message); err != nil {
			return err
		}
	}
	return nil
}

func StatusText(writer io.Writer, report model.StatusReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon status"); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Workspace: %s", report.WorkspaceRoot); err != nil {
		return err
	}
	if err := out.line("Initialized: %t", report.Initialized); err != nil {
		return err
	}
	if report.ManifestPath != "" {
		if err := out.line("Manifest: %s", report.ManifestPath); err != nil {
			return err
		}
	}
	if report.SyncStatePath != "" {
		if err := out.line("Sync state: %s", report.SyncStatePath); err != nil {
			return err
		}
	}
	if err := out.line("Summary: manifest=%t syncState=%t baseClaude=%t baseCodex=%t baseCanon=%t open=%d resolved=%d warnings=%d", report.Summary.HasManifest, report.Summary.HasSyncState, report.Summary.HasBaseClaude, report.Summary.HasBaseCodex, report.Summary.HasBaseCanon, report.Summary.OpenConflicts, report.Summary.ResolvedConflicts, report.Summary.Warnings); err != nil {
		return err
	}
	if len(report.Warnings) == 0 {
		return nil
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Warnings:"); err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		message, _ := security.RedactContent(warning.Message)
		if err := out.line("- warning[%s]: %s", warning.Code, message); err != nil {
			return err
		}
	}
	return nil
}

func DiffText(writer io.Writer, report model.DiffReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon diff %s", report.Target); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Summary: diffs=%d open=%d resolved=%d warnings=%d", report.Summary.Diffs, report.Summary.OpenConflicts, report.Summary.ResolvedConflicts, report.Summary.Warnings); err != nil {
		return err
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Diffs:"); err != nil {
		return err
	}
	if len(report.Diffs) == 0 {
		if err := out.line("- none"); err != nil {
			return err
		}
	}
	for _, diff := range report.Diffs {
		summary, _ := security.RedactContent(diff.Summary)
		if err := out.line("- %s [%s] %s: %s", diff.DiffKind, diff.Scope, diff.ResourceID, summary); err != nil {
			return err
		}
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Conflicts:"); err != nil {
		return err
	}
	if len(report.Conflicts) == 0 {
		if err := out.line("- none"); err != nil {
			return err
		}
	}
	for _, conflict := range report.Conflicts {
		if err := out.line("- %s %s %s %s [%s]", conflict.Status, conflict.ID, conflict.Kind, conflict.ResourceID, conflict.ResourceKind); err != nil {
			return err
		}
	}
	if len(report.Warnings) == 0 {
		return nil
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Warnings:"); err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		message, _ := security.RedactContent(warning.Message)
		if err := out.line("- warning[%s]: %s", warning.Code, message); err != nil {
			return err
		}
	}
	return nil
}

func SyncText(writer io.Writer, report model.SyncStateReport, workspaceRoot string, statePath string) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon sync: %s -> %s", report.Source, report.Target); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Workspace: %s", workspaceRoot); err != nil {
		return err
	}
	if err := out.line("State: %s", statePath); err != nil {
		return err
	}
	if err := out.line("Summary: diffs=%d open=%d resolved=%d warnings=%d", report.Summary.Diffs, report.Summary.OpenConflicts, report.Summary.ResolvedConflicts, report.Summary.Warnings); err != nil {
		return err
	}
	return writeWarnings(out, report.Warnings)
}

func ConflictsText(writer io.Writer, report model.SyncStateReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon conflicts: %s -> %s", report.Source, report.Target); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Summary: open=%d resolved=%d diffs=%d warnings=%d", report.Summary.OpenConflicts, report.Summary.ResolvedConflicts, report.Summary.Diffs, report.Summary.Warnings); err != nil {
		return err
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Open conflicts:"); err != nil {
		return err
	}
	firstOpenID := ""
	firstOpenHasSuggestion := false
	for _, conflict := range report.Conflicts {
		if conflict.Status != model.ConflictStatusOpen {
			continue
		}
		if firstOpenID == "" {
			firstOpenID = conflict.ID
			firstOpenHasSuggestion = conflict.Suggestion != ""
		}
		if err := out.line("- %s %s %s [%s] scope=%s", conflict.ID, conflict.Kind, conflict.ResourceID, conflict.ResourceKind, conflict.Scope); err != nil {
			return err
		}
		if err := out.line("  why: %s", conflictReason(conflict.Kind)); err != nil {
			return err
		}
		for _, summary := range []string{
			conflictSideSummary("base", conflict.Base),
			conflictSideSummary("ours", conflict.Ours),
			conflictSideSummary("theirs", conflict.Theirs),
		} {
			if err := out.line("  %s", summary); err != nil {
				return err
			}
		}
		if conflict.Suggestion != "" {
			if err := out.line("  suggestion: confidence=%.2f", conflict.SuggestionConfidence); err != nil {
				return err
			}
		}
	}
	if firstOpenID == "" {
		if err := out.line("- none"); err != nil {
			return err
		}
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Resolved conflicts: %d", report.Summary.ResolvedConflicts); err != nil {
		return err
	}
	if firstOpenID != "" {
		if err := writeConflictNextSteps(out, report.Source, report.Target, firstOpenID, firstOpenHasSuggestion); err != nil {
			return err
		}
	}
	return writeWarnings(out, report.Warnings)
}

func conflictReason(kind model.ConflictKind) string {
	switch kind {
	case model.ConflictKindSecurity:
		return "security-sensitive resource or redacted secret changed"
	case model.ConflictKindCapability:
		return "unsupported capability, hook, or session resource changed"
	case model.ConflictKindContent:
		return "both sides changed content differently"
	case model.ConflictKindLocation:
		return "resource location changed"
	case model.ConflictKindSemantic:
		return "semantic meaning differs"
	default:
		return "review both sides before resolving"
	}
}

func conflictSideSummary(label string, state *model.ResourceState) string {
	if state == nil {
		return fmt.Sprintf("%s: missing", label)
	}
	path, _ := security.RedactContent(state.Path)
	if path == "" {
		path = "none"
	}
	strategy, _ := security.RedactContent(state.Strategy)
	if strategy == "" {
		strategy = "none"
	}
	return fmt.Sprintf("%s: hash=%s path=%s status=%s strategy=%s", label, conflictHash(state.ContentHash), path, state.Status, strategy)
}

func conflictHash(hash string) string {
	if hash == "" {
		return "none"
	}
	if len(hash) > 24 {
		return hash[:24]
	}
	return hash
}

func writeConflictNextSteps(out textWriter, source string, target string, conflictID string, hasSuggestion bool) error {
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Next steps:"); err != nil {
		return err
	}
	if err := out.line("- Keep %s side: `agent-canon resolve %s --ours`", toolLabel(source), conflictID); err != nil {
		return err
	}
	if err := out.line("- Keep %s side: `agent-canon resolve %s --theirs`", toolLabel(target), conflictID); err != nil {
		return err
	}
	if hasSuggestion {
		if err := out.line("- Accept suggestion: `agent-canon resolve %s --accept-suggestion`", conflictID); err != nil {
			return err
		}
	}
	return out.line("- Write a manual value: `agent-canon resolve %s --manual <value>`", conflictID)
}

func toolLabel(tool string) string {
	switch tool {
	case "claude":
		return "Claude"
	case "codex":
		return "Codex"
	default:
		return tool
	}
}

func ResolveText(writer io.Writer, conflictID string, decision model.ResolutionDecision, resolutionID string) error {
	out := textWriter{writer: writer}
	return out.line("resolved %s with %s as %s", conflictID, decision, resolutionID)
}

func ImportText(writer io.Writer, report model.ImportReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon import %s", report.Tool); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Workspace: %s", report.WorkspaceRoot); err != nil {
		return err
	}
	if err := out.line("Snapshot: %s", report.SnapshotPath); err != nil {
		return err
	}
	if err := out.line("Report: %s", report.ReportPath); err != nil {
		return err
	}
	if err := out.line("Summary: resources=%d warnings=%d", report.Summary.Resources, report.Summary.Warnings); err != nil {
		return err
	}
	if len(report.Warnings) == 0 {
		return nil
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Warnings:"); err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		message, _ := security.RedactContent(warning.Message)
		if err := out.line("- warning[%s]: %s", warning.Code, message); err != nil {
			return err
		}
	}
	return nil
}

type ApplyTextReport struct {
	Target        string
	Project       string
	Mode          string
	IncludeGlobal bool
	BackupDir     string
	ManifestPath  string
	Changes       []model.ApplyFileChange
	Warnings      []model.Warning
}

func ApplyText(writer io.Writer, report ApplyTextReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon apply %s: %s", report.Target, report.Mode); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	create, modify, noop := applyActionCounts(report.Changes)
	if err := out.line("Summary: create=%d modify=%d noop=%d warnings=%d", create, modify, noop, len(report.Warnings)); err != nil {
		return err
	}
	if report.Mode == "dry-run" {
		if err := out.line("Dry-run: no files were written."); err != nil {
			return err
		}
		if err := out.line("Backup and rollback manifest are created only when apply runs without --dry-run."); err != nil {
			return err
		}
		if report.IncludeGlobal {
			if err := out.line("Global boundary: listed global paths point at real Claude/Codex homes, but dry-run does not write them."); err != nil {
				return err
			}
		} else {
			if err := out.line("Global boundary: global Claude/Codex home writes are intentionally excluded unless --global is used."); err != nil {
				return err
			}
		}
	}
	if report.BackupDir != "" {
		if err := out.line("Backup: %s", report.BackupDir); err != nil {
			return err
		}
	}
	if report.ManifestPath != "" {
		if err := out.line("Manifest: %s", report.ManifestPath); err != nil {
			return err
		}
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Changed files:"); err != nil {
		return err
	}
	if len(report.Changes) == 0 {
		if err := out.line("- none"); err != nil {
			return err
		}
	}
	for _, change := range report.Changes {
		if err := out.line("- %s [%s] %s", change.Action, change.Scope, change.Path); err != nil {
			return err
		}
	}
	if report.Mode == "dry-run" {
		if err := writeApplyDryRunNextSteps(out, report); err != nil {
			return err
		}
	}
	if len(report.Warnings) == 0 {
		return nil
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Warnings:"); err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		message, _ := security.RedactContent(warning.Message)
		if err := out.line("- warning[%s]: %s", warning.Code, message); err != nil {
			return err
		}
	}
	return nil
}

func writeApplyDryRunNextSteps(out textWriter, report ApplyTextReport) error {
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Next steps:"); err != nil {
		return err
	}
	if err := out.line("- Review Changed files before any write."); err != nil {
		return err
	}
	command := fmt.Sprintf("agent-canon apply %s --yes", report.Target)
	if report.IncludeGlobal {
		command = fmt.Sprintf("agent-canon apply %s --global --yes", report.Target)
	}
	if err := out.line("- Run `%s` only after dry-run looks correct.", command); err != nil {
		return err
	}
	if hasWarningCode(report.Warnings, "global-skipped") {
		if err := out.line("- To inspect skipped global home changes first, run `agent-canon apply %s --global --dry-run`.", report.Target); err != nil {
			return err
		}
	}
	return nil
}

func hasWarningCode(warnings []model.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

type RollbackTextReport struct {
	Target       string
	Project      string
	Mode         string
	BackupDir    string
	ManifestPath string
	Changes      []model.ApplyFileChange
	Warnings     []model.Warning
}

func RollbackText(writer io.Writer, report RollbackTextReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon rollback %s: %s", report.Target, report.Mode); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	restore, deleteCount, noop := rollbackActionCounts(report.Changes)
	if err := out.line("Summary: restore=%d delete=%d noop=%d warnings=%d", restore, deleteCount, noop, len(report.Warnings)); err != nil {
		return err
	}
	if report.BackupDir != "" {
		if err := out.line("Backup: %s", report.BackupDir); err != nil {
			return err
		}
	}
	if report.ManifestPath != "" {
		if err := out.line("Manifest: %s", report.ManifestPath); err != nil {
			return err
		}
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Rollback changes:"); err != nil {
		return err
	}
	if len(report.Changes) == 0 {
		if err := out.line("- none"); err != nil {
			return err
		}
	}
	for _, change := range report.Changes {
		if err := out.line("- %s [%s] %s", rollbackOperation(change.Action), change.Scope, change.Path); err != nil {
			return err
		}
	}
	if len(report.Warnings) == 0 {
		return nil
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Warnings:"); err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		message, _ := security.RedactContent(warning.Message)
		if err := out.line("- warning[%s]: %s", warning.Code, message); err != nil {
			return err
		}
	}
	return nil
}

func VerifyText(writer io.Writer, report model.VerifyReport) error {
	out := textWriter{writer: writer}
	if err := out.line("agent-canon verify %s", report.Target); err != nil {
		return err
	}
	if err := out.line("Project: %s", report.Project); err != nil {
		return err
	}
	if err := out.line("Summary: pass=%d warn=%d fail=%d warnings=%d", report.Summary.Pass, report.Summary.Warn, report.Summary.Fail, len(report.Warnings)); err != nil {
		return err
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Checks:"); err != nil {
		return err
	}
	if len(report.Checks) == 0 {
		if err := out.line("- none"); err != nil {
			return err
		}
	}
	for _, check := range report.Checks {
		message, _ := security.RedactContent(check.Message)
		path := ""
		if check.Path != "" {
			path = fmt.Sprintf(" (%s)", check.Path)
		}
		if err := out.line("- %s %s: %s%s", check.Status, check.ID, message, path); err != nil {
			return err
		}
	}
	if len(report.Warnings) == 0 {
		return nil
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Warnings:"); err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		message, _ := security.RedactContent(warning.Message)
		if err := out.line("- warning[%s]: %s", warning.Code, message); err != nil {
			return err
		}
	}
	return nil
}

func writeWarnings(out textWriter, warnings []model.Warning) error {
	if len(warnings) == 0 {
		return nil
	}
	if err := out.blank(); err != nil {
		return err
	}
	if err := out.line("Warnings:"); err != nil {
		return err
	}
	for _, warning := range warnings {
		message, _ := security.RedactContent(warning.Message)
		if err := out.line("- warning[%s]: %s", warning.Code, message); err != nil {
			return err
		}
	}
	return nil
}

func applyActionCounts(changes []model.ApplyFileChange) (int, int, int) {
	create := 0
	modify := 0
	noop := 0
	for _, change := range changes {
		switch change.Action {
		case model.ApplyActionCreate:
			create++
		case model.ApplyActionModify:
			modify++
		case model.ApplyActionNoop:
			noop++
		}
	}
	return create, modify, noop
}

func rollbackActionCounts(changes []model.ApplyFileChange) (int, int, int) {
	restore := 0
	deleteCount := 0
	noop := 0
	for _, change := range changes {
		switch change.Action {
		case model.ApplyActionCreate:
			deleteCount++
		case model.ApplyActionModify:
			restore++
		case model.ApplyActionNoop:
			noop++
		}
	}
	return restore, deleteCount, noop
}

func rollbackOperation(action model.ApplyAction) string {
	switch action {
	case model.ApplyActionCreate:
		return "delete"
	case model.ApplyActionModify:
		return "restore"
	case model.ApplyActionNoop:
		return "noop"
	default:
		return string(action)
	}
}

func actionDescription(action string) string {
	switch action {
	case "create-or-merge":
		return "These operations are candidates for generated preview files."
	case "manual":
		return "Review these operations before trusting generated output."
	case "skip":
		return "These operations are intentionally not written by agent-canon."
	case "redact":
		return "These operations contain redacted sensitive content and require manual review."
	default:
		return "Review these operations before applying changes."
	}
}

func actionOrder(operations []model.Operation) []string {
	preferred := []string{"create-or-merge", "manual", "skip", "redact"}
	seen := make(map[string]bool)
	actions := make([]string, 0, len(preferred))
	for _, action := range preferred {
		for _, operation := range operations {
			if operation.Action == action && !seen[action] {
				seen[action] = true
				actions = append(actions, action)
			}
		}
	}
	for _, operation := range operations {
		if operation.Action == "" || seen[operation.Action] {
			continue
		}
		seen[operation.Action] = true
		actions = append(actions, operation.Action)
	}
	if len(actions) == 0 {
		return []string{"Operations"}
	}
	return actions
}

type textWriter struct {
	writer io.Writer
}

func (tw textWriter) blank() error {
	return tw.write("\n")
}

func (tw textWriter) line(format string, args ...any) error {
	if len(args) == 0 {
		return tw.write(format + "\n")
	}
	return tw.write(fmt.Sprintf(format+"\n", args...))
}

func (tw textWriter) write(text string) error {
	if _, err := io.Copy(tw.writer, strings.NewReader(text)); err != nil {
		return fmt.Errorf("write text: %w", err)
	}
	return nil
}
