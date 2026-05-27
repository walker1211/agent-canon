package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

func ScanJSON(writer io.Writer, report model.ScanReport) error {
	return writeJSON(writer, report)
}

func PlanJSON(writer io.Writer, report model.PlanReport) error {
	return writeJSON(writer, report)
}

func InitJSON(writer io.Writer, report model.WorkspaceManifestReport) error {
	return writeJSON(writer, report)
}

func StatusJSON(writer io.Writer, report model.StatusReport) error {
	return writeJSON(writer, report)
}

func DiffJSON(writer io.Writer, report model.DiffReport) error {
	return writeJSON(writer, report)
}

func SyncJSON(writer io.Writer, report model.SyncStateReport) error {
	return writeJSON(writer, report)
}

func VerifyJSON(writer io.Writer, report model.VerifyReport) error {
	return writeJSON(writer, report)
}

func writeJSON(writer io.Writer, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	payload = append(payload, '\n')
	if _, err := writer.Write(payload); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}
