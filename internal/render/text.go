package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
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
	groups := []struct {
		title  string
		status model.Status
	}{
		{title: "Compatible", status: model.StatusCompatible},
		{title: "Partial", status: model.StatusPartial},
		{title: "Unsupported", status: model.StatusUnsupported},
		{title: "Dangerous", status: model.StatusDangerous},
	}
	for _, group := range groups {
		if err := out.blank(); err != nil {
			return err
		}
		if err := out.line("%s:", group.title); err != nil {
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

	actions := actionOrder(report.Operations)
	for _, action := range actions {
		if err := out.blank(); err != nil {
			return err
		}
		if err := out.line("%s:", action); err != nil {
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
		if err := out.line("- %s %s", operation.ID, operation.ResourceID); err != nil {
			return err
		}
	}
	if !found {
		return out.line("- none")
	}
	return nil
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
