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

func TestHigoLuaRunExecutesShebangScript(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "tool.lua")
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env lua\nreturn 'cli-shebang'"), 0o700); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./cmd/higoluarun", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "cli-shebang" {
		t.Fatalf("output = %q, want cli-shebang", got)
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

func TestHigoLuaRunExecutesInlineChunk(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/higoluarun", "-e", `return arg[0] .. ":" .. arg[1] .. ":" .. (3 * 7)`, "inline")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun -e failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "-e:inline:21" {
		t.Fatalf("output = %q, want -e:inline:21", got)
	}
}

func TestHigoLuaRunArgTableReportsArgumentCount(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/higoluarun", "-e", `return arg[0] .. ":" .. #arg .. ":" .. tostring(arg.n) .. ":" .. arg[1] .. ":" .. arg[2]`, "left", "right")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun -e failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "-e:2:2:left:right" {
		t.Fatalf("output = %q, want arg count metadata", got)
	}
}

func TestHigoLuaRunHonorsOsExitCode(t *testing.T) {
	bin := buildHigoLuaRun(t)

	okCmd := exec.Command(bin, "-e", `os.exit(0)`)
	if output, err := okCmd.CombinedOutput(); err != nil {
		t.Fatalf("higoluarun os.exit(0) failed: %v\n%s", err, output)
	}

	failCmd := exec.Command(bin, "-e", `os.exit(7)`)
	output, err := failCmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("higoluarun os.exit(7) error = %T %v, want exit error\n%s", err, err, output)
	}
	if exitErr.ExitCode() != 7 {
		t.Fatalf("exit code = %d, want 7\n%s", exitErr.ExitCode(), output)
	}

	pcallCmd := exec.Command(bin, "-e", `pcall(function() os.exit(7) end)`)
	output, err = pcallCmd.CombinedOutput()
	exitErr, ok = err.(*exec.ExitError)
	if !ok {
		t.Fatalf("higoluarun pcall os.exit(7) error = %T %v, want exit error\n%s", err, err, output)
	}
	if exitErr.ExitCode() != 7 {
		t.Fatalf("pcall exit code = %d, want 7\n%s", exitErr.ExitCode(), output)
	}

	xpcallCmd := exec.Command(bin, "-e", `xpcall(function() os.exit(7) end, function(err) return err end)`)
	output, err = xpcallCmd.CombinedOutput()
	exitErr, ok = err.(*exec.ExitError)
	if !ok {
		t.Fatalf("higoluarun xpcall os.exit(7) error = %T %v, want exit error\n%s", err, err, output)
	}
	if exitErr.ExitCode() != 7 {
		t.Fatalf("xpcall exit code = %d, want 7\n%s", exitErr.ExitCode(), output)
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

func TestHigoLuaRunTestHonorsOsExitCodes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exit_zero.lua"), []byte(`os.exit(0)`), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "./cmd/higoluarun", "test", dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("higoluarun test failed for os.exit(0): %v\n%s", err, output)
	}
	if got := string(output); !strings.Contains(got, "PASS exit_zero.lua") {
		t.Fatalf("output = %q, want PASS for os.exit(0)", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "exit_nonzero.lua"), []byte(`os.exit(5)`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "run", "./cmd/higoluarun", "test", dir)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("higoluarun test succeeded, want os.exit(5) failure\n%s", output)
	}
	got := string(output)
	if !strings.Contains(got, "PASS exit_zero.lua") || !strings.Contains(got, "FAIL exit_nonzero.lua") || !strings.Contains(got, "os.exit(5)") {
		t.Fatalf("output = %q, want os.exit test pass/fail lines", got)
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

func buildHigoLuaRun(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "higoluarun")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/higoluarun")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build higoluarun failed: %v\n%s", err, output)
	}
	return bin
}
