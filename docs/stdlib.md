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
  `getfenv`/`setfenv` support functions and Lua stack levels)
- `table`: `table.insert`, `table.remove`, `table.sort`, `table.concat`
  with Lua 5.1 sparse insert/remove boundary behavior, default concat
  separator, raw integer-key access, and explicit concat range validation,
  `table.unpack`, `table.getn`, `table.maxn` over numeric keys,
  `table.foreach`,
  `table.foreachi`
- `string`: `string.len`, `string.upper`, `string.lower`, `string.byte`,
  `string.char`, `string.reverse`, `string.sub`, `string.rep`,
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
  (`file:read` supports `*l`, `*a`, `*n`, and byte counts)
- `os`: `os.time`, `os.date`, `os.clock`, `os.difftime`, `os.getenv`,
  `os.tmpname`, `os.rename`, `os.remove`, `os.execute`, `os.setlocale`,
  `os.exit`
- `package`: `require`, `package.path`, `package.loaded`,
  `package.preload`, `package.loaders`, `package.searchers`,
  `package.cpath`, `package.config`, `package.seeall`, `module`,
  unsupported `package.loadlib`
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
classes. `string.gsub` supports string, table, and function replacements.
`string.format` accepts Lua integer-style specifiers such as `%d`, `%i`, `%o`,
`%u`, `%x`, `%X`, and `%c` by coercing Lua numbers to integers before passing
them to the formatter.
Advanced Lua pattern features such as embedded balanced items are still
remaining compatibility items.

`string.dump` produces a State-local pure-Go function token that `load` and
`loadstring` can load again inside the same State. It is intentionally not a Lua
C bytecode blob and is not portable across State instances.
