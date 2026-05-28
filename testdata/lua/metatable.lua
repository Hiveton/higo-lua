local fallback = {name = "fallback"}
local t = setmetatable({}, {
  __index = fallback,
  __newindex = function(target, key, value)
    rawset(target, key .. "_seen", value .. ":ok")
  end,
  __call = function(self, value)
    return self.name .. ":" .. value
  end,
})

t.result = "write"

if t.name ~= "fallback" then
  error("__index fallback mismatch")
end

if t.result_seen ~= "write:ok" then
  error("__newindex function mismatch")
end

if t("call") ~= "fallback:call" then
  error("__call mismatch")
end

local protected = {}
setmetatable(protected, {__metatable = "locked"})
if getmetatable(protected) ~= "locked" then
  error("__metatable protection mismatch")
end

