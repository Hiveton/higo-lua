prefix = "legacy"
module("compat.sample", package.seeall)

value = prefix .. ":" .. _NAME .. ":" .. _PACKAGE

if value ~= "legacy:compat.sample:compat." then
	error("module value mismatch: " .. tostring(value))
end

if package.loaded["compat.sample"] ~= _M then
	error("module not cached")
end
