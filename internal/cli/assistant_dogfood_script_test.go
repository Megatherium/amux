package cli

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantDogfoodScript_MissingFlagValueFailsClearly(t *testing.T) {
	requireBinary(t, "bash")

	scriptPath := filepath.Join("..", "..", "skills", "amux", "scripts", "assistant-dogfood.sh")
	cmd := exec.Command(scriptPath, "--repo")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for missing flag value")
	}
	text := string(out)
	if !strings.Contains(text, "missing value for --repo") {
		t.Fatalf("output = %q, want missing flag guidance", text)
	}
}
