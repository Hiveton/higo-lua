#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT"

go test ./...
go test -race ./...

go run ./cmd/higoluarun ./testdata/lua/basic.lua
inline_output="$(go run ./cmd/higoluarun -e 'return arg[0] .. ":" .. #arg .. ":" .. arg.n .. ":" .. arg[1] .. ":" .. (3 * 7)' inline)"
if [[ "$inline_output" != "-e:1:1:inline:21" ]]; then
	echo "inline CLI output = $inline_output, want -e:1:1:inline:21" >&2
	exit 1
fi
stdin_output="$(printf 'return arg[0] .. ":" .. #arg .. ":" .. arg.n .. ":" .. arg[1]\n' | go run ./cmd/higoluarun - cliarg)"
if [[ "$stdin_output" != "-:1:1:cliarg" ]]; then
	echo "stdin CLI output = $stdin_output, want -:1:1:cliarg" >&2
	exit 1
fi
runner_bin="$(mktemp "$ROOT/higoluarun.XXXXXX")"
go build -o "$runner_bin" ./cmd/higoluarun
"$runner_bin" -e 'os.exit(0)'
set +e
"$runner_bin" -e 'os.exit(7)' >/tmp/higoluarun-exit.out 2>&1
exit_code=$?
set -e
if [[ "$exit_code" -ne 7 ]]; then
	echo "os.exit CLI code = $exit_code, want 7" >&2
	rm -f /tmp/higoluarun-exit.out "$runner_bin"
	exit 1
fi
set +e
"$runner_bin" -e 'pcall(function() os.exit(7) end)' >/tmp/higoluarun-exit.out 2>&1
pcall_exit_code=$?
"$runner_bin" -e 'xpcall(function() os.exit(7) end, function(err) return err end)' >/tmp/higoluarun-exit.out 2>&1
xpcall_exit_code=$?
set -e
rm -f /tmp/higoluarun-exit.out
if [[ "$pcall_exit_code" -ne 7 ]]; then
	echo "pcall os.exit CLI code = $pcall_exit_code, want 7" >&2
	exit 1
fi
if [[ "$xpcall_exit_code" -ne 7 ]]; then
	echo "xpcall os.exit CLI code = $xpcall_exit_code, want 7" >&2
	exit 1
fi
exit_test_dir="$(mktemp -d)"
printf 'os.exit(0)\n' >"$exit_test_dir/exit_zero.lua"
exit_test_output="$("$runner_bin" test "$exit_test_dir")"
if [[ "$exit_test_output" != *"PASS exit_zero.lua"* ]]; then
	echo "higoluarun test os.exit(0) output = $exit_test_output" >&2
	rm -rf "$exit_test_dir"
	exit 1
fi
printf 'os.exit(5)\n' >"$exit_test_dir/exit_nonzero.lua"
set +e
exit_test_output="$("$runner_bin" test "$exit_test_dir" 2>&1)"
exit_test_code=$?
set -e
rm -rf "$exit_test_dir"
rm -f "$runner_bin"
if [[ "$exit_test_code" -eq 0 || "$exit_test_output" != *"FAIL exit_nonzero.lua"* || "$exit_test_output" != *"os.exit(5)"* ]]; then
	echo "higoluarun test nonzero os.exit code = $exit_test_code output = $exit_test_output" >&2
	exit 1
fi
go run ./cmd/higoluarun test ./testdata/lua

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

cd "$tmp"
go mod init external-check >/dev/null
cat > main.go <<'GO'
package main

import (
	"context"
	"fmt"

	higolua "github.com/Hiveton/higo-lua"
	"github.com/Hiveton/higo-lua/state"
	"github.com/Hiveton/higo-lua/value"
)

func main() {
	runtimeValue, err := higolua.NewRuntime().DoString(context.Background(), `return 1 + 2`)
	if err != nil {
		panic(err)
	}

	st := state.New()
	defer st.Close()
	st.Register("double", func(ctx context.Context, args state.Args) (value.Value, error) {
		return value.Number(args.Number(0) * 2), nil
	})
	if err := st.DoString(context.Background(), `result = double(21)`); err != nil {
		panic(err)
	}
	goValue, err := st.GetGlobal("result")
	if err != nil {
		panic(err)
	}

	fmt.Println(runtimeValue.String() + ":" + goValue.String())
}
GO
go mod edit -replace github.com/Hiveton/higo-lua="$ROOT"
go mod tidy
external_output="$(go run .)"
if [[ "$external_output" != "3:42" ]]; then
	echo "external import smoke output = $external_output, want 3:42" >&2
	exit 1
fi
