# HiGoLua Architecture

HiGoLua is a pure Go Lua 5.1 runtime library. The public API is split between
`github.com/Hiveton/higo-lua` for simple one-shot execution and
`github.com/Hiveton/higo-lua/state` for embedded hosts that need globals, Go
function registration, and Go-to-Lua calls.

The default engine now attempts bytecode execution first for the verified
subset and falls back to the AST interpreter when a chunk uses Lua surface area
that the compiler has not lowered yet. The public API intentionally does not
expose AST, bytecode, or interpreter types, so the engine can keep expanding
without breaking callers.
When a debug hook is active, State intentionally uses the AST interpreter so
`line`, `count`, `call`, and `return` events are delivered consistently.

`internal/bytecode` and `internal/vm` now contain the second-stage compiler/VM
boundary. The bytecode path currently supports a small verified subset
(`return`, constants, local assignment/name reads, arithmetic, power, modulo,
division, concatenation, comparisons, unary `-`/`not`/`#`, `if`/`else`,
`while`, `repeat`/`until`, `break`, numeric `for` with positive and negative
steps, generic `for` with VM closure iterators and VM-safe `pairs`/`ipairs`
expression expansion, lexical local scopes including `do ... end`, global
read/write through the State environment, table constructors, table
indexing/assignment including `__index` table fallback and `__newindex`
table/function forwarding, `__call` table invocation, arithmetic metamethods
`__add`/`__sub`/`__mul`/`__div`/`__mod`/`__pow`, comparison/equality
metamethods `__lt`/`__le`/`__eq`, concatenation metamethod `__concat`,
unary metamethods `__len`/`__unm`, `and`/`or` short-circuiting, and internal
Lua closure creation/calls including dotted function declarations and
local/global table-field closures invoked with colon method syntax, first-value
vararg, and the Lua 5.1 `arg` table,
mutable captured locals/upvalues, plus chunk-level multiple return values
through `ExecuteValues`, including final-call expansion in return lists and
assignment lists and final-field expansion in table constructors) and is
exercised by unit tests.
The VM also has a caller hook used by `state.State`, so VM-compatible chunks
can directly call Go functions explicitly registered by the host with
`Register` or `RegisterMulti`; the State layer also marks `pairs`, `ipairs`,
`next`, `setmetatable`, `getmetatable`, `rawget`, `rawset`, `type`,
`tostring`, `tonumber`, `assert`, `error`, `rawequal`, `collectgarbage`, and
`gcinfo` as VM-safe host calls. String-literal and local-string colon methods such as
`("abc"):sub(1, 2)` and `s:rep(2)` can also run through the bytecode path by
looking up methods on the standard `string` table. Loaded standard library
tables (`string`, `math`, `table`, `package`, `coroutine`, `io`, `os`, and
`debug`) are exposed to the compiler as known global tables, so direct calls
such as `string.sub(...)`, `math.max(...)`, and `table.concat(...)` can compile
when their callee values are otherwise VM-callable. Other standard library
calls still fall back when they rely on AST-only runtime behavior.
Chunks that expose VM closures back through the public `State` API still fall
back to the AST path, preserving Go/Lua interop while the VM closure type is
kept internal. Multi-return chunks are now represented in the VM result path,
and final function calls in VM return lists expand to all returned values.

Default `state.New()` and `higolua.NewRuntime()` load the full Lua-style
standard profile. `stdlib.Safe()` is available for embedded hosts that want to
disable `io`, `os`, `debug`, and filesystem chunk loading through `loadfile`,
`dofile`, and the default package file loader while retaining
`package.preload` for in-memory modules.

Lua C modules are intentionally unsupported. `package.loadlib` returns a clear
error because this runtime is pure Go.

Implemented compatibility highlights:

- `function` declarations and closures.
- numeric `for`, Lua iterator-protocol generic `for`, `pairs`/`ipairs`,
  `while`, `repeat`, `if`, and lexical `do ... end` blocks.
- multiple return values in Lua function calls and assignments.
- vararg functions with `...`, Lua 5.1 `arg`/`arg.n`, and `select`.
- `pcall` status plus result/error propagation.
- Lua 5.1 multi-return adjustment for assignment, return lists, function
  arguments, and final table-constructor fields.
- `loadstring`, `loadfile`, and `dofile` preserve chunk return values.
- table constructors, table indexing, `rawget`, `rawset`, `setmetatable`,
  `getmetatable`, `__index` table fallback, `__newindex` table/function
  assignment forwarding, `__metatable` protection, and `__call` table
  invocation.
- metatable binary operations for `__add`, `__sub`, `__mul`, `__div`,
  `__mod`, `__pow`, `__concat`, `__lt`, and `__le`.
- metatable equality/string/length/unary conversion through `__eq`,
  `__tostring`, `__len`, and `__unm`.
- colon method definition/call syntax, lowered to explicit `self`.
- string colon methods such as `("abc"):sub(1, 2)` through the standard
  `string` table.
- debug environment and upvalue helpers for Lua closures through `debug.getfenv`,
  `debug.setfenv`, `debug.getupvalue`, `debug.setupvalue`, `debug.getlocal`,
  `debug.setlocal`, `debug.sethook`, and `debug.gethook`.
