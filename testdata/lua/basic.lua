local sum = 0
for i = 1, 3 do
  sum = sum + i
end
if .5 + 1 ~= 1.5 then
  error("leading dot number mismatch")
end
if [[
hello]] ~= "hello" then
  error("long string initial newline mismatch")
end
local typed_keys = {}
typed_keys[true] = "bool"
typed_keys["true"] = "string"
typed_keys[1.5] = "number"
typed_keys["1.5"] = "string-number"
if typed_keys[true] ~= "bool" or typed_keys["true"] ~= "string" or typed_keys[1.5] ~= "number" or typed_keys["1.5"] ~= "string-number" then
  error("table key type mismatch")
end
local next_values = {}
next_values[2] = "two"
next_values[1] = nil
local next_key, next_value = next(next_values)
if next_key ~= 2 or next_value ~= "two" or next(next_values, next_key) ~= nil then
  error("next nil-slot mismatch")
end
local invalid_key_values = {}
local nil_key_ok = pcall(function()
  invalid_key_values[nil] = "bad"
end)
local nan_key_ok = pcall(function()
  invalid_key_values[0 / 0] = "bad"
end)
if nil_key_ok or nan_key_ok or next(invalid_key_values) ~= nil then
  error("invalid table key mismatch")
end
local length_values = {"a", "b", "c"}
length_values[3] = nil
if #length_values ~= 2 or table.getn(length_values) ~= 2 or table.concat(length_values, "") ~= "ab" then
  error("trailing nil length mismatch")
end
table.insert(length_values, "d")
if #length_values ~= 3 or table.concat(length_values, "") ~= "abd" then
  error("table.insert append length mismatch")
end
length_values[3] = nil
local removed_tail = table.remove(length_values)
if removed_tail ~= "b" or #length_values ~= 1 or table.concat(length_values, "") ~= "a" then
  error("table.remove sequence length mismatch")
end
return sum
