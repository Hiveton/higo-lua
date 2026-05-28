# HiGoLua

HiGoLua is a pure Go Lua 5.1 runtime library designed for embedding in other
Go projects.

```go
rt := higolua.NewRuntime()
result, err := rt.DoString(context.Background(), `return 1 + 2 * 3`)
```

Run the library checks:

```bash
scripts/verify.sh
```

The verification script runs the full package test suite, race tests, CLI Lua
script execution, directory-based Lua compatibility scripts, stdin execution,
and an external Go module smoke test that imports `github.com/Hiveton/higo-lua`.

Individual checks:

```bash
go test ./...
go test -race ./...
go run ./cmd/higoluarun test ./testdata/lua
go run ./cmd/higoluarun -e 'return 1 + 2'
go run ./cmd/higoluarun ./testdata/lua/basic.lua
echo 'return "stdin"' | go run ./cmd/higoluarun -
```

The `cmd/higoluarun` command runs external Lua files directly from this module.
It also has a `test <directory>` mode for executing compatibility scripts under
`testdata/lua`. The runner adds the script directory to `package.path`, so
single-file tools can require helper modules placed beside the entry script.
Use `-e <chunk>` for inline Lua snippets.

See `docs/api.md` and `docs/architecture.md`.

Current status:

- Public Runtime and State APIs are usable from other Go projects.
- The default AST interpreter covers core Lua control flow, closures, tables,
  Go/Lua calls, varargs, multiple returns, `pcall`, metatable `__index`,
  string colon methods, chunk loading functions, and a growing standard
  library subset.
- Go/Lua embedding supports both single-value and multi-value calls through
  `Register` / `Call` and `RegisterMulti` / `CallValues`.
- VM-compatible chunks can call host-registered Go functions through the
  bytecode caller hook; `pairs`, `ipairs`, `setmetatable`, `getmetatable`,
  `next`, `rawget`, `rawset`, `type`, `tostring`, `tonumber`, `assert`,
  `error`, `rawequal`, `collectgarbage`, and `gcinfo` are also VM-safe host
  calls.
  Loaded standard library tables such as `string`, `math`, and `table` can be
  indexed by bytecode-compatible chunks.
- Public error aliases cover syntax, runtime, Go bridge, and context
  cancellation failures.
- The CLI compatibility corpus under `testdata/lua` covers direct execution,
  closures/varargs, multi-return adjustment, metatables, modules, core
  standard libraries, and `io`/`os` smoke behavior.
- The default engine attempts bytecode first for verified VM-compatible chunks
  and falls back to the AST interpreter for unsupported Lua surface area.
