prefix = "legacy"
module("compat.sample", package.seeall)

value = prefix .. ":" .. _NAME .. ":" .. _PACKAGE

if value ~= "legacy:compat.sample:compat." then
	error("module value mismatch: " .. tostring(value))
end

if package.loaded["compat.sample"] ~= _M then
	error("module not cached")
end

local load_count = 0
package.preload.false_reload = function(name)
	load_count = load_count + 1
	return {count = load_count, name = name}
end

local first = require("false_reload")
package.loaded.false_reload = false
local second = require("false_reload")

if first == second or first.count ~= 1 or second.count ~= 2 or second.name ~= "false_reload" then
	error("package.loaded false reload mismatch")
end
