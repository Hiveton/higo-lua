local function pair()
  return "B", "C"
end

local a, b, c = "A", pair()
if a ~= "A" or b ~= "B" or c ~= "C" then
  error("local assignment multi-return adjustment mismatch")
end

a = nil
b = nil
c = nil
a, b, c = "A", pair()
if a ~= "A" or b ~= "B" or c ~= "C" then
  error("assignment multi-return adjustment mismatch")
end

local results = {pair()}
if results[1] ~= "B" or results[2] ~= "C" then
  error("table constructor multi-return mismatch")
end

local x, y, z = (function()
  return "A", pair()
end)()

if x ~= "A" or y ~= "B" or z ~= "C" then
  error("return-list final call expansion mismatch")
end
