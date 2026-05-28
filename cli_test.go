package higolua_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHigoLuaRunExecutesExternalScriptAndModuleDirectory(t *testing.T) {
	dir := t.TempDir()
	moduleDir := filepath.Join(dir, "lib")
	if err := os.Mkdir(moduleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "greeter.lua"), []byte(`
local greeter = {}
function greeter.message(name)
  return "hello " .. name
end
return greeter
`), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "main.lua")
	if err := os.WriteFile(scriptPath, []byte(`
package.path = arg[1]
local greeter = require("greeter")
return greeter.message("external")
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./cmd/higoluarun", scriptPath, filepath.ToSlash(filepath.Join(moduleDir, "?.lua")))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "hello external" {
		t.Fatalf("output = %q, want hello external", got)
	}
}

func TestHigoLuaRunAutomaticallyRequiresModulesBesideScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.lua"), []byte(`
return {
  value = function()
    return "beside-script"
  end
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "main.lua")
	if err := os.WriteFile(scriptPath, []byte(`
local helper = require("helper")
return helper.value()
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./cmd/higoluarun", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "beside-script" {
		t.Fatalf("output = %q, want beside-script", got)
	}
}

func TestHigoLuaRunPrintsMultipleReturnValues(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "multi.lua")
	if err := os.WriteFile(scriptPath, []byte(`return "left", 42, true`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./cmd/higoluarun", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "left\t42\ttrue" {
		t.Fatalf("output = %q, want tab-separated multiple returns", got)
	}
}

func TestHigoLuaRunExecutesStdinChunk(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/higoluarun", "-", "from-stdin")
	cmd.Stdin = strings.NewReader(`return arg[0] .. ":" .. arg[1]`)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun stdin failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "-:from-stdin" {
		t.Fatalf("output = %q, want -:from-stdin", got)
	}
}

func TestHigoLuaRunTestRunsDirectoryScripts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pass.lua"), []byte(`
if 1 + 2 ~= 3 then
  error("math failed")
end
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./cmd/higoluarun", "test", dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun test failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); !strings.Contains(got, "PASS") || !strings.Contains(got, "pass.lua") {
		t.Fatalf("output = %q, want PASS line for pass.lua", got)
	}
}

func TestHigoLuaRunTestReportsFailingScripts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.lua"), []byte(`error("boom")`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./cmd/higoluarun", "test", dir)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("higoluarun test succeeded, want failure\n%s", output)
	}
	got := string(output)
	if !strings.Contains(got, "FAIL bad.lua") || !strings.Contains(got, "boom") {
		t.Fatalf("output = %q, want failing file and error message", got)
	}
}
