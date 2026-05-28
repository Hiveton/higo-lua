# HiGoLua API

## One-shot Runtime

```go
rt := higolua.NewRuntime()
result, err := rt.DoString(ctx, `return 1 + 2`)
result, err = rt.DoFile(ctx, "main.lua")
result, err = rt.DoReader(ctx, "reader.lua", strings.NewReader(`return "ok"`))
```

`Runtime` creates an isolated state for each call.
`DoFile` also prepends the script directory to `package.path`, so a top-level
script can `require` sibling Lua modules without changing the host process
working directory.

## Embedded State

```go
st := state.New()
defer st.Close()

st.SetGlobal("name", value.String("HiGo"))
st.SetGlobal("hostObject", value.NewUserData(myObject))
st.Register("add", func(ctx context.Context, args state.Args) (value.Value, error) {
    return value.Number(args.Number(0) + args.Number(1)), nil
})
st.RegisterMulti("split", func(ctx context.Context, args state.Args) ([]value.Value, error) {
    return []value.Value{value.String("left"), value.String("right")}, nil
})
err := st.RegisterModule("host", map[string]state.GoFunc{
    "upper": func(ctx context.Context, args state.Args) (value.Value, error) {
        return value.String(strings.ToUpper(args.String(0))), nil
    },
})

err := st.DoString(ctx, `result = add(20, 22)`)
v, err := st.GetGlobal("result")
v, err = st.Call(ctx, "lua_func", value.String("input"))
values, err := st.CallValues(ctx, "lua_multi_func")
```

`State` keeps globals and loaded functions alive until `Close`.
`State.DoFile` prepends the script directory to that state's `package.path`
before executing the chunk, matching the one-shot `Runtime.DoFile` behavior for
external scripts with sibling modules.

Use `Register` / `Call` for single-value Go/Lua calls. Use `RegisterMulti` /
`CallValues` when the Lua call boundary must preserve all returned values.
Use `RegisterModule` to expose a Go-backed Lua library through
`require("module_name")`.

Use `value.NewUserData(any)` to pass opaque host objects into Lua as
`userdata`. Lua code can inspect them with `type`, and metatables can be
attached through the debug library when that library is enabled.

Use `state.WithStdlib(stdlib.Safe())` when embedding in a sandboxed host. The
safe profile keeps the base, package, coroutine, table, string, and math
libraries while omitting `io`, `os`, and `debug`. It also disables filesystem
chunk loading through `loadfile`, `dofile`, and the default package file
loader; `package.preload` remains available for in-memory modules.

## Errors

APIs return stable error types that work with `errors.As`:

- `higolua.SyntaxError` / `state.SyntaxError`
- `higolua.RuntimeError` / `state.RuntimeError`
- `higolua.BridgeError` / `state.BridgeError`
- `higolua.ContextError` / `state.ContextError`

The error string remains the underlying Lua-facing message; `Unwrap` exposes the
original error for callers that need lower-level details.

`SyntaxError` carries `Chunk`, `Line`, and `Column`. `RuntimeError` carries
`Chunk`, `Line`, `Column`, and a basic Lua function call stack. `BridgeError`
keeps the registered Go function name and, when available from the statement
boundary, the Lua source position that triggered it.
