package infra

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOverrideEscapesComposeInterpolation(t *testing.T) {
	p := Plan{StateDir: t.TempDir(), Slug: "test"}
	path, err := p.writeOverride([]Service{{Compose: "on", Env: map[string]string{"true": "pa$word", "OTHER": "x${Y}", "BROKEN": "${", "DOUBLE": "$$"}}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`  "on":`, `      "true": "pa$$word"`, `      "OTHER": "x$${Y}"`, `      "BROKEN": "$${"`, `      "DOUBLE": "$$$$"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("override missing %q:\n%s", want, data)
		}
	}
}

func TestOverrideComposeConfig(t *testing.T) {
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		t.Skip("Docker Compose CLI unavailable:", err)
	}
	p := Plan{StateDir: t.TempDir(), Slug: "test"}
	env := map[string]string{"PASSWORD": "pa$word", "BRACED": "x${Y}", "UNFINISHED": "${", "DOUBLE": "$$", "ESCAPES": "a\n\"b\\c", "true": "false", "EMPTY": ""}
	path, err := p.writeOverride([]Service{{Compose: "on", Env: env}})
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(base, []byte("services:\n  \"on\":\n    image: example.invalid/test:latest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("docker", "compose", "-p", "fride-test", "-f", base, "-f", path, "config", "--format", "json")
	cmd.Env = append(os.Environ(), "word=HOST-VALUE", "Y=HOST-VALUE")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compose config: %v\n%s", err, out)
	}
	var config struct {
		Services map[string]struct{ Environment map[string]string }
	}
	if err := json.Unmarshal(out, &config); err != nil {
		t.Fatal(err)
	}
	// Compose re-escapes literal dollars when exporting a reusable config.
	// Host variable substitution or invalid interpolation would change these values
	// or cause the command above to fail.
	want := map[string]string{"PASSWORD": "pa$$word", "BRACED": "x$${Y}", "UNFINISHED": "$${", "DOUBLE": "$$$$", "ESCAPES": "a\n\"b\\c", "true": "false", "EMPTY": ""}
	if got := config.Services["on"].Environment; !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
	}
}
