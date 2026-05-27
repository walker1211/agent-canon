package apply

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/zhangyoujun/agent-canon/internal/exporter"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

type CodexPlanInput struct {
	Scan          model.ScanReport
	Plan          model.PlanReport
	IncludeGlobal bool
}

type CodexPlan struct {
	Changes  []FileChange
	Warnings []model.Warning
}

type FileChange struct {
	model.ApplyFileChange
	PreviewPath string `json:"previewPath,omitempty"`
	Contents    []byte `json:"-"`
}

func BuildCodexPlan(input CodexPlanInput) (CodexPlan, error) {
	var out CodexPlan
	out.Warnings = append(out.Warnings, input.Scan.Warnings...)
	out.Warnings = append(out.Warnings, input.Plan.Warnings...)

	projectScan := scanForScope(input.Scan, model.ScopeProject)
	if len(projectScan.Resources) > 0 {
		changes, err := changesForScope(projectScan, planForScan(input.Plan, projectScan), input.Scan.Project, model.ScopeProject)
		if err != nil {
			return CodexPlan{}, err
		}
		out.Changes = append(out.Changes, changes...)
	}

	globalScan := scanForScope(input.Scan, model.ScopeGlobal)
	if len(globalScan.Resources) > 0 {
		if input.IncludeGlobal {
			changes, err := changesForScope(globalScan, planForScan(input.Plan, globalScan), input.Scan.CodexHome, model.ScopeGlobal)
			if err != nil {
				return CodexPlan{}, err
			}
			out.Changes = append(out.Changes, changes...)
		} else {
			out.Warnings = append(out.Warnings, model.Warning{Code: "global-skipped", Message: "global Codex targets were skipped; rerun with --global to include --codex-home writes"})
		}
	}

	sort.SliceStable(out.Changes, func(i, j int) bool {
		return out.Changes[i].Path < out.Changes[j].Path
	})
	return out, nil
}

func changesForScope(scan model.ScanReport, plan model.PlanReport, root string, scope model.Scope) ([]FileChange, error) {
	preview, err := exporter.BuildCodexPreview(scan, plan)
	if err != nil {
		return nil, err
	}
	changes := make([]FileChange, 0, len(preview.Files))
	for _, file := range preview.Files {
		if file.Path == "migration-report.md" {
			continue
		}
		path := targetPath(root, scope, file.Path)
		change, err := buildFileChange(path, scope, file.Path, file.Contents)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func targetPath(root string, scope model.Scope, previewPath string) string {
	clean := filepath.FromSlash(previewPath)
	if scope != model.ScopeGlobal {
		return filepath.Join(root, clean)
	}
	switch clean {
	case filepath.Join(".codex", "config.toml"):
		return filepath.Join(root, "config.toml")
	case "AGENTS.md":
		return filepath.Join(root, "AGENTS.md")
	}
	if rest, ok := trimPathPrefix(clean, filepath.Join(".agents", "skills")); ok {
		return filepath.Join(root, "skills", rest)
	}
	if rest, ok := trimPathPrefix(clean, filepath.Join(".codex", "agents")); ok {
		return filepath.Join(root, "agents", rest)
	}
	return filepath.Join(root, clean)
}

func trimPathPrefix(path string, prefix string) (string, bool) {
	rel, err := filepath.Rel(prefix, path)
	if err != nil || rel == "." || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(os.PathSeparator) {
		return "", false
	}
	return rel, true
}

func buildFileChange(path string, scope model.Scope, previewPath string, contents []byte) (FileChange, error) {
	afterHash := hashBytes(contents)
	change := FileChange{
		ApplyFileChange: model.ApplyFileChange{
			Path:      path,
			Scope:     scope,
			Action:    model.ApplyActionCreate,
			AfterHash: afterHash,
		},
		PreviewPath: previewPath,
		Contents:    append([]byte(nil), contents...),
	}

	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return change, nil
	}
	if err != nil {
		return FileChange{}, fmt.Errorf("read apply target %s: %w", path, err)
	}
	change.BeforeHash = hashBytes(current)
	if change.BeforeHash == afterHash {
		change.Action = model.ApplyActionNoop
		return change, nil
	}
	change.Action = model.ApplyActionModify
	return change, nil
}

func hashBytes(contents []byte) string {
	sum := sha256.Sum256(contents)
	return fmt.Sprintf("sha256:%x", sum)
}

func scanForScope(scan model.ScanReport, scope model.Scope) model.ScanReport {
	out := scan
	out.Resources = nil
	for _, resource := range scan.Resources {
		if resource.Scope == scope {
			out.Resources = append(out.Resources, resource)
		}
	}
	return out
}

func planForScan(plan model.PlanReport, scan model.ScanReport) model.PlanReport {
	ids := make(map[string]bool, len(scan.Resources))
	for _, resource := range scan.Resources {
		ids[resource.ID] = true
	}
	out := plan
	out.Operations = nil
	for _, operation := range plan.Operations {
		if ids[operation.ResourceID] {
			out.Operations = append(out.Operations, operation)
		}
	}
	return out
}
