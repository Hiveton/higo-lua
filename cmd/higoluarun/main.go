package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hiveton/higolua/state"
	"github.com/hiveton/higolua/value"
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
	if args[0] == "test" {
		if len(args) != 2 {
			return usage()
		}
		return runTestDir(ctx, args[1])
	}
	return runScript(ctx, args[0], args[1:], true)
}

func usage() error {
	return fmt.Errorf("usage: higoluarun <script.lua> [args...] | higoluarun test <directory>")
}

func runScript(ctx context.Context, scriptPath string, scriptArgs []string, printResult bool) error {
	st := state.New()
	defer st.Close()
	st.SetGlobal("arg", argTable(scriptPath, scriptArgs))
	result, err := st.DoFile(ctx, scriptPath)
	if err != nil {
		return err
	}
	if printResult && result != value.Nil {
		fmt.Println(result.String())
	}
	return nil
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
