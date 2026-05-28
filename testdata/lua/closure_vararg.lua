local function make_counter(start)
  local value = start
  return function(...)
    local delta = select(1, ...)
    value = value + delta
    return value, arg[1], arg.n
  end
end

local counter = make_counter(10)
local a, seen, count = counter(5)
local b = counter(7)

if a ~= 15 or seen ~= 5 or count ~= 1 or b ~= 22 then
  error("closure/vararg mismatch")
end
