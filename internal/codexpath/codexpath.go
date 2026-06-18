package codexpath

import "path/filepath"

// UserSkillsRoot returns the Codex user-level skills directory documented as
// $HOME/.agents/skills. Derive HOME from CODEX_HOME for existing CLI plumbing.
func UserSkillsRoot(codexHome string) string {
	return filepath.Join(filepath.Dir(filepath.Clean(codexHome)), ".agents", "skills")
}
