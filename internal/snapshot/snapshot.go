package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

type Set struct {
	Claude model.SnapshotReport
	Codex  model.SnapshotReport
	Canon  model.CanonSnapshotReport
}

func Build(scan model.ScanReport) (Set, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339)
	claudeResources, claudeWarnings, err := buildClaudeStates(scan.Resources)
	if err != nil {
		return Set{}, err
	}
	codexResources, codexWarnings, err := buildCodexStates(scan)
	if err != nil {
		return Set{}, err
	}
	canonResources, canonWarnings, err := buildCanonStates(scan.Resources)
	if err != nil {
		return Set{}, err
	}

	return Set{
		Claude: model.SnapshotReport{
			SchemaVersion: model.SnapshotSchemaVersion,
			Tool:          "claude",
			CreatedAt:     createdAt,
			Project:       scan.Project,
			Resources:     claudeResources,
			Warnings:      appendWarnings(scan.Warnings, claudeWarnings...),
		},
		Codex: model.SnapshotReport{
			SchemaVersion: model.SnapshotSchemaVersion,
			Tool:          "codex",
			CreatedAt:     createdAt,
			Project:       scan.Project,
			Resources:     codexResources,
			Warnings:      appendWarnings(scan.Warnings, codexWarnings...),
		},
		Canon: model.CanonSnapshotReport{
			SchemaVersion: model.CanonSnapshotSchemaVersion,
			CreatedAt:     createdAt,
			Project:       scan.Project,
			Resources:     canonResources,
			Warnings:      appendWarnings(scan.Warnings, canonWarnings...),
		},
	}, nil
}

func buildClaudeStates(resources []model.Resource) ([]model.ResourceState, []model.Warning, error) {
	states := make([]model.ResourceState, 0, len(resources))
	var warnings []model.Warning
	for _, resource := range resources {
		state := resourceState(resource, "claude", resource.SourcePath)
		if storesSourceContent(resource.Kind) {
			stateWarnings, err := addNormalizedFileContent(&state, resource.SourcePath)
			if err != nil {
				return nil, nil, err
			}
			warnings = append(warnings, stateWarnings...)
		}
		states = append(states, state)
	}
	sortStates(states)
	return states, warnings, nil
}

func buildCodexStates(scan model.ScanReport) ([]model.ResourceState, []model.Warning, error) {
	var states []model.ResourceState
	var warnings []model.Warning
	seen := map[string]bool{}
	for _, resource := range scan.Resources {
		for _, path := range codexTargetPaths(scan, resource) {
			key := resource.ID + "\x00" + path
			if seen[key] {
				continue
			}
			seen[key] = true
			if !regularFileExists(path) {
				continue
			}
			state := resourceState(resource, "codex", path)
			state.TargetPathHint = path
			stateWarnings, err := addNormalizedFileContent(&state, path)
			if err != nil {
				return nil, nil, err
			}
			warnings = append(warnings, stateWarnings...)
			states = append(states, state)
		}
	}
	sortStates(states)
	return states, warnings, nil
}

func codexTargetPaths(scan model.ScanReport, resource model.Resource) []string {
	var paths []string
	if resource.TargetPathHint != "" {
		paths = append(paths, resource.TargetPathHint)
	}
	if configPath := codexConfigTargetPath(scan, resource); configPath != "" {
		paths = append(paths, configPath)
	}
	for _, warning := range resource.Warnings {
		if path := existingCodexTargetPath(warning); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func codexConfigTargetPath(scan model.ScanReport, resource model.Resource) string {
	if resource.Kind != model.KindConfig {
		return ""
	}
	if resource.Scope == model.ScopeProject || resource.Scope == model.ScopeLocal {
		return filepath.Join(scan.Project, ".codex", "config.toml")
	}
	return filepath.Join(scan.CodexHome, "config.toml")
}

func existingCodexTargetPath(warning model.Warning) string {
	if warning.Code != "existing-codex-target" {
		return ""
	}
	const prefix = "Codex target already exists and should be reviewed before merging:"
	if !strings.HasPrefix(warning.Message, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(warning.Message, prefix))
}

func buildCanonStates(resources []model.Resource) ([]model.ResourceState, []model.Warning, error) {
	states := make([]model.ResourceState, 0, len(resources))
	for _, resource := range resources {
		states = append(states, model.ResourceState{
			ID:          resource.ID,
			Kind:        resource.Kind,
			Scope:       resource.Scope,
			Tool:        "canon",
			Status:      resource.Status,
			Strategy:    resource.Strategy,
			ContentHash: canonIdentityHash(resource),
			Warnings:    []model.Warning{},
		})
	}
	sortStates(states)
	return states, nil, nil
}

func canonIdentityHash(resource model.Resource) string {
	identity := strings.Join([]string{resource.ID, string(resource.Kind), string(resource.Scope), string(resource.Status), resource.Strategy}, "\n")
	hash := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(hash[:])
}

func resourceState(resource model.Resource, tool string, path string) model.ResourceState {
	return model.ResourceState{
		ID:             resource.ID,
		Kind:           resource.Kind,
		Scope:          resource.Scope,
		Tool:           tool,
		Path:           path,
		TargetPathHint: resource.TargetPathHint,
		Status:         resource.Status,
		Strategy:       resource.Strategy,
		Warnings:       append([]model.Warning{}, resource.Warnings...),
	}
}

func addNormalizedFileContent(state *model.ResourceState, path string) ([]model.Warning, error) {
	contents, ok, err := readRegularFile(path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	normalized := NormalizeContent(contents)
	state.NormalizedText = normalized.Text
	state.ContentHash = normalized.ContentHash
	if !normalized.SecretRedacted {
		return nil, nil
	}
	warning := model.Warning{Code: "secret-redacted", Message: "resource content contained a redacted secret"}
	state.Warnings = appendWarningIfMissing(state.Warnings, warning)
	return []model.Warning{warning}, nil
}

func readRegularFile(path string) ([]byte, bool, error) {
	if path == "" {
		return nil, false, nil
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("stat snapshot file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read snapshot file %s: %w", path, err)
	}
	return contents, true, nil
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func storesSourceContent(kind model.ResourceKind) bool {
	switch kind {
	case model.KindInstruction, model.KindRule, model.KindSkill, model.KindCommand:
		return true
	default:
		return false
	}
}

func appendWarnings(existing []model.Warning, additions ...model.Warning) []model.Warning {
	out := append([]model.Warning{}, existing...)
	for _, warning := range additions {
		out = appendWarningIfMissing(out, warning)
	}
	return out
}

func appendWarningIfMissing(warnings []model.Warning, warning model.Warning) []model.Warning {
	for _, existing := range warnings {
		if existing.Code == warning.Code && existing.Message == warning.Message {
			return warnings
		}
	}
	return append(warnings, warning)
}

func sortStates(states []model.ResourceState) {
	sort.SliceStable(states, func(i, j int) bool {
		if states[i].ID != states[j].ID {
			return states[i].ID < states[j].ID
		}
		if states[i].Tool != states[j].Tool {
			return states[i].Tool < states[j].Tool
		}
		return states[i].Path < states[j].Path
	})
}
