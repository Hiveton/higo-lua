# Standard Library Compatibility

The default profile opens these modules:

- `base`: `print`, `type`, `tostring`, `tonumber`, `load`, `loadstring`,
  `loadfile`, `dofile`, `next`, `unpack`, `rawequal`, `collectgarbage`,
  `gcinfo`, `newproxy`
- `base`: `assert`, `error`, `pcall`, `xpcall`, `pairs`, `ipairs`, `rawget`,
  `rawset`, `setmetatable`, `getmetatable`, `getfenv`, `setfenv`, `_G`,
  `select`
  (`tonumber` supports bases 2 through 36; `load` supports string chunks and
  reader functions; `setmetatable`/`getmetatable` honor `__metatable`;
  `getfenv`/`setfenv` support functions and Lua stack levels; `ipairs`
  performs raw array reads; `unpack` preserves nil values inside explicit
  ranges)
- `table`: `table.insert`, `table.remove`, `table.sort`, `table.concat`
  with Lua 5.1 sparse insert/remove boundary behavior, default sorting through
  Lua `<` semantics including `__lt`, default concat separator, raw
  integer-key access, and explicit concat range validation,
  `table.unpack`, `table.getn`, `table.maxn` over numeric keys,
  `table.foreach`, `table.foreachi` with raw array reads
- `string`: `string.len`, `string.upper`, `string.lower`, `string.byte`
  with zero results for empty ranges,
  `string.char`, `string.reverse`, `string.sub` with Lua 5.1 end-index
  defaults, `string.rep` with empty output for non-positive counts,
  `string.find`, `string.match`, `string.gmatch`, `string.gsub`,
  `string.format`, `string.dump`
- `math`: `math.pi`, `math.floor`, `math.ceil`, `math.sqrt`, `math.abs`,
  `math.max`, `math.min`, `math.random`, `math.randomseed`, `math.huge`,
  `math.sin`, `math.cos`, `math.tan`, `math.asin`, `math.acos`,
  `math.atan`, `math.atan2`, `math.sinh`, `math.cosh`, `math.tanh`,
  `math.deg`, `math.rad`, `math.exp`, `math.log`, `math.log10`,
  `math.pow`, `math.fmod`, `math.mod`, `math.frexp`, `math.ldexp`,
  `math.modf`
- `io`: `io.read`, `io.write`, `io.flush`, `io.input`, `io.output`,
  `io.close`, `io.open`, `io.popen`, `io.lines`, `io.tmpfile`, `io.type`,
  `io.stdin`, `io.stdout`, `io.stderr`, file handles with `read`, `write`,
  `flush`, `close`, `lines`, `seek`, and `setvbuf`
  (`file:read` supports `*l`, `*a`, `*n`, and byte counts; `io.lines()`
  without a path iterates the current default input without closing it)
- `os`: `os.time` with local-time table input, `os.date` with explicit
  timestamps, UTC `!`, `*t`, weekday/month names, weekday/day-of-year, and
  12-hour/AM-PM plus common numeric format specifiers, `os.clock`,
  `os.difftime`, `os.getenv`,
  `os.tmpname`, `os.rename`, `os.remove`, `os.execute`, `os.setlocale`,
  `os.exit` (`os.rename` and `os.remove` return `nil, message` on failure)
- `package`: `require`, `package.path`, `package.loaded`,
  `package.preload`, `package.loaders`, `package.searchers`,
  `package.cpath`, `package.config`, `package.seeall`, `module`,
  unsupported `package.loadlib` (`require` only treats truthy
  `package.loaded` entries as cached)
- `debug`: `debug.traceback`, `debug.getinfo`, `debug.getmetatable`,
  `debug.setmetatable`, `debug.getregistry`, `debug.getfenv`,
  `debug.setfenv`, `debug.getupvalue`, `debug.setupvalue`,
  `debug.getlocal`, `debug.setlocal`, `debug.sethook`, `debug.gethook`
  (`debug.traceback` includes the active Lua function stack; hooks dispatch
  `call`, `return`, `line`, and count events in the AST interpreter)
- `coroutine`: `coroutine.create`, `coroutine.resume`, `coroutine.yield`,
  `coroutine.status`, `coroutine.running`, `coroutine.wrap`

`stdlib.Safe()` keeps the base, package, coroutine, table, string, and math
libraries, disables `io`, `os`, and `debug`, and blocks filesystem chunk
loading through `loadfile`, `dofile`, and the default package file loader.
`package.preload` remains available for in-memory modules.

This is a first pure-Go compatibility layer, not a C Lua runtime. The known
gap is Lua C dynamic loading.

The compatibility table is intentionally conservative. Functions not listed
above still need implementation and tests before they should be treated as
supported.

`string.find`, `string.match`, `string.gmatch`, and `string.gsub` support the
common Lua pattern classes and captures through a pure-Go regexp-backed
implementation, with a dedicated scanner for simple `%bxy` balanced matches.
Frontier assertions through `%f[...]` are supported for common byte-oriented
classes, and the Lua non-greedy `-` repetition modifier is honored.
`string.gsub` supports string, table, and function replacements.
`string.format` accepts Lua integer-style specifiers such as `%d`, `%i`, `%o`,
`%u`, `%x`, `%X`, and `%c` by coercing Lua numbers to integers before passing
them to the formatter, and ignores extra arguments that are not consumed by
the format string. Missing format arguments raise a Lua runtime error.
Advanced Lua pattern features such as embedded balanced items are still
remaining compatibility items.

`string.dump` produces a State-local pure-Go function token that `load` and
`loadstring` can load again inside the same State. It is intentionally not a Lua
C bytecode blob and is not portable across State instances.
