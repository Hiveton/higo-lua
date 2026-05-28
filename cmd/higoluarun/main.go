package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Hiveton/higo-lua/state"
	"github.com/Hiveton/higo-lua/value"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usage()
	}
	if args[0] == "-e" {
		if len(args) < 2 {
			return usage()
		}
		return runInline(ctx, args[1], args[2:], true)
	}
	if args[0] == "test" {
		if len(args) != 2 {
			return usage()
		}
		return runTestDir(ctx, args[1])
	}
	return runScript(ctx, args[0], args[1:], true)
}

func usage() error {
	return fmt.Errorf("usage: higoluarun <script.lua|-> [args...] | higoluarun -e <chunk> [args...] | higoluarun test <directory>")
}

func runInline(ctx context.Context, source string, scriptArgs []string, printResult bool) error {
	st := state.New()
	defer st.Close()
	st.SetGlobal("arg", argTable("-e", scriptArgs))
	if err := prependScriptPackagePath(ctx, st, "."); err != nil {
		return err
	}
	results, err := st.DoChunkValues(ctx, "-e", source)
	if err != nil {
		return err
	}
	if printResult {
		printResults(results)
	}
	return nil
}

func runScript(ctx context.Context, scriptPath string, scriptArgs []string, printResult bool) error {
	st := state.New()
	defer st.Close()
	st.SetGlobal("arg", argTable(scriptPath, scriptArgs))
	if err := prependScriptPackagePath(ctx, st, scriptPath); err != nil {
		return err
	}
	source, err := readScriptSource(scriptPath)
	if err != nil {
		return err
	}
	results, err := st.DoChunkValues(ctx, scriptPath, string(source))
	if err != nil {
		return err
	}
	if printResult {
		printResults(results)
	}
	return nil
}

func readScriptSource(scriptPath string) (string, error) {
	if scriptPath == "-" {
		data, err := io.ReadAll(os.Stdin)
		return string(data), err
	}
	data, err := os.ReadFile(scriptPath)
	return string(data), err
}

func prependScriptPackagePath(ctx context.Context, st *state.State, scriptPath string) error {
	dir := filepath.Dir(scriptPath)
	if scriptPath == "-" {
		dir = "."
	}
	patterns := []string{
		filepath.ToSlash(filepath.Join(dir, "?.lua")),
		filepath.ToSlash(filepath.Join(dir, "?", "init.lua")),
	}
	source := fmt.Sprintf("package.path = %q .. ';' .. package.path", strings.Join(patterns, ";"))
	return st.DoString(ctx, source)
}

func printResults(results []value.Value) {
	if len(results) == 0 {
		return
	}
	rendered := make([]string, 0, len(results))
	for _, result := range results {
		if result == nil || result == value.Nil {
			continue
		}
		rendered = append(rendered, result.String())
	}
	if len(rendered) > 0 {
		fmt.Println(strings.Join(rendered, "\t"))
	}
}

func runTestDir(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var scripts []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lua") {
			continue
		}
		scripts = append(scripts, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(scripts)
	if len(scripts) == 0 {
		return fmt.Errorf("no .lua scripts found in %s", dir)
	}
	var failed bool
	for _, script := range scripts {
		if err := runScript(ctx, script, nil, false); err != nil {
			failed = true
			fmt.Printf("FAIL %s: %v\n", filepath.Base(script), err)
			continue
		}
		fmt.Printf("PASS %s\n", filepath.Base(script))
	}
	if failed {
		return fmt.Errorf("one or more Lua scripts failed")
	}
	return nil
}

func argTable(scriptPath string, scriptArgs []string) *value.Table {
	tab := value.NewTable()
	tab.Set(value.Number(0), value.String(scriptPath))
	for i, arg := range scriptArgs {
		tab.Set(value.Number(i+1), value.String(arg))
	}
	return tab
}
