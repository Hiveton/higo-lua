local text = string.gsub("a1 b2", "%f[%d]%d", function(d)
  return tostring(tonumber(d) + 1)
end)

if text ~= "a2 b3" then
  error("string.gsub/frontier mismatch: " .. text)
end
local gsub_bad_capture_ok = pcall(function()
  return string.gsub("abc", "a", "%1")
end)
local gsub_whole = string.gsub("abc", "a", "[%0]%%")
if gsub_bad_capture_ok or gsub_whole ~= "[a]%bc" then
  error("string.gsub replacement capture mismatch")
end

if string.format("%d:%i:%x:%X:%o:%c:%u:%08u", 7.9, -3.2, 255, 255, 9, 65, -1, 15) ~= "7:-3:ff:FF:11:A:4294967295:00000015" then
  error("string.format integer coercion mismatch")
end

local assert_a, assert_b, assert_c = assert("ok", "left", "right")
if assert_a ~= "ok" or assert_b ~= "left" or assert_c ~= "right" then
  error("assert multi-return mismatch")
end

local xpcall_ok, xpcall_arg = xpcall(function(arg)
  return tostring(arg)
end, function(err)
  return "handled:" .. err
end, "extra")
if not xpcall_ok or xpcall_arg ~= "nil" then
  error("xpcall argument compatibility mismatch")
end
local function select_count(...)
  return select("#", ...)
end
if select_count(select(4, "a", "b")) ~= 0 then
  error("select out-of-range multi-return mismatch")
end

local values = {3, 1, 2}
table.sort(values)
if table.concat(values, ",") ~= "1,2,3" then
  error("table.sort/concat mismatch")
end
local maxn_values = {}
maxn_values[5] = "present"
local maxn_before = table.maxn(maxn_values)
maxn_values[5] = nil
maxn_values[3] = "left"
if maxn_before ~= 5 or table.maxn(maxn_values) ~= 3 then
  error("table.maxn deleted-slot mismatch")
end
local numeric_values = {10, 2, 1}
table.sort(numeric_values)
if table.concat(numeric_values, ",") ~= "1,2,10" then
  error("table.sort numeric order mismatch")
end
local concat_bool_ok = pcall(function()
  return table.concat({"a", true}, "")
end)
local concat_table_ok = pcall(function()
  return table.concat({"a", {}}, "")
end)
if concat_bool_ok or concat_table_ok then
  error("table.concat element compatibility mismatch")
end
local remove_values = {"a", "b"}
local remove_ok = pcall(function()
  return table.remove(remove_values, 0)
end)
local remove_far_ok = pcall(function()
  return table.remove(remove_values, 3)
end)
local remove_empty_explicit_ok = pcall(function()
  return table.remove({}, 1)
end)
if remove_ok or remove_far_ok or remove_empty_explicit_ok or table.remove(remove_values) ~= "b" then
  error("table.remove position compatibility mismatch")
end
local insert_values = {"a", "b"}
local insert_zero_ok = pcall(function()
  table.insert(insert_values, 0, "bad")
end)
local insert_far_ok = pcall(function()
  table.insert(insert_values, 5, "bad")
end)
table.insert(insert_values, "c")
if insert_zero_ok or insert_far_ok or table.concat(insert_values, "") ~= "abc" then
  error("table.insert position compatibility mismatch")
end

local n = math.max(1, 9, 3) + math.min(8, 4)
if n ~= 13 then
  error("math max/min mismatch")
end

local co_create_ok = pcall(function()
  return coroutine.create(123)
end)
local co_wrap_ok = pcall(function()
  return coroutine.wrap("bad")
end)
if co_create_ok or co_wrap_ok then
  error("coroutine function argument mismatch")
end

local registry = debug.getregistry()
registry.sample = "registry-ok"
if debug.getregistry().sample ~= "registry-ok" then
  error("debug registry mismatch")
end
local info = debug.getinfo(probe_info or function() return 1 end)
if info.what ~= "Lua" or info.linedefined < 1 then
  error("debug getinfo source position mismatch")
end

if string.sub(package.config, 1, 1) ~= "/" or string.sub(package.config, 3, 3) ~= ";" then
  error("package.config mismatch")
end

package.searchers = {
  function(name)
    if name == "searcher_runtime" then
      return function(module_name)
        return {value = "searcher:" .. module_name}
      end
    end
    return "\n\tsearcher missed"
  end
}
local searcher_mod = require("searcher_runtime")
if searcher_mod.value ~= "searcher:searcher_runtime" then
  error("package.searchers require mismatch")
end

local secret = "up"
local function probe()
  local local_value = "local"
  local lname, lvalue = debug.getlocal(1, 1)
  if not lname or not lvalue then
    error("debug.getlocal mismatch")
  end
  return secret .. ":" .. local_value
end
local secret_index = nil
local i = 1
while true do
  local uname, uvalue = debug.getupvalue(probe, i)
  if not uname then
    break
  end
  if uname == "secret" and uvalue == "up" then
    secret_index = i
    break
  end
  i = i + 1
end
if not secret_index then
  error("debug.getupvalue mismatch")
end
debug.setupvalue(probe, secret_index, "changed")
if probe() ~= "changed:local" then
  error("debug.setupvalue mismatch")
end

local dump_secret = "dump"
local function dumped(name)
  return dump_secret .. ":" .. name
end
local loaded_dump = assert(loadstring(string.dump(dumped)))
if loaded_dump("ok") ~= "dump:ok" then
  error("string.dump/loadstring mismatch")
end

local hook_events = {}
debug.sethook(function(event)
  hook_events[event] = true
end, "crl", 2)
local function hooked()
  local n = 1
  n = n + 1
  return n
end
hooked()
debug.sethook()
if not (hook_events.call and hook_events["return"] and hook_events.line and hook_events.count) then
  error("debug hook dispatch mismatch")
end
