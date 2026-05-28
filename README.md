# HiGoLua

HiGoLua is a pure Go Lua 5.1 runtime library designed for embedding in other
Go projects.

```go
rt := higolua.NewRuntime()
result, err := rt.DoString(context.Background(), `return 1 + 2 * 3`)
```

Run the library checks:

```bash
go test ./...
go test -race ./...
(cd ../runtime-examples && go run ./cmd/higoluarun test ../higolua/testdata/lua)
```

The CLI and runnable integration examples live in the separate
`../runtime-examples` module. That project imports this library through the
public `github.com/hiveton/higolua` API.

See `docs/api.md` and `docs/architecture.md`.

Current status:

- Public Runtime and State APIs are usable from other Go projects.
- `Runtime.DoFile` and `State.DoFile` can run external Lua scripts that
  `require` sibling Lua modules from the script directory.
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
