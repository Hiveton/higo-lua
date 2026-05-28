local path = os.tmpname()
local file, open_err = io.open(path, "w+")
if not file then
  error("io.open failed: " .. tostring(open_err))
end

file:write("HiGoLua")
file:seek("set", 1)
local data = file:read("*a")
file:close()
os.remove(path)

if data ~= "iGoLua" then
  error("io read/seek mismatch: " .. tostring(data))
end

local number_path = os.tmpname()
local number_file = assert(io.open(number_path, "w+"))
number_file:write("  -12.5 1.25e2 0x10 rest")
number_file:seek("set", 0)
local number_value = number_file:read("*n")
local number_exp = number_file:read("*n")
local number_hex = number_file:read("*n")
local number_rest = number_file:read("*l")
number_file:close()
os.remove(number_path)

if number_value ~= -12.5 or number_exp ~= 125 or number_hex ~= 16 or number_rest ~= " rest" then
  error("io read number mismatch")
end

local multi_path = os.tmpname()
local multi_file = assert(io.open(multi_path, "w+"))
multi_file:write("12 rest\nnext\n")
multi_file:seek("set", 0)
local multi_number, multi_line, multi_all = multi_file:read("*number", "*line", "*all")
multi_file:close()
os.remove(multi_path)

if multi_number ~= 12 or multi_line ~= " rest" or multi_all ~= "next\n" then
  error("io read aliases/multiple formats mismatch")
end

local global_path = os.tmpname()
local global_file = assert(io.open(global_path, "w+"))
global_file:write("34 more\nleft\n")
global_file:seek("set", 0)
local previous_input = io.input(global_file)
local global_number, global_line, global_all = io.read("*number", "*line", "*all")
io.input(previous_input)
global_file:close()
os.remove(global_path)

if global_number ~= 34 or global_line ~= " more" or global_all ~= "left\n" then
  error("global io.read multiple formats mismatch")
end

local all_path = os.tmpname()
local all_file = assert(io.open(all_path, "w+"))
all_file:write("a\nb\n")
all_file:seek("set", 0)
local all_data = all_file:read("*a")
all_file:close()
os.remove(all_path)

if all_data ~= "a\nb\n" then
  error("io read all newline mismatch")
end

if type(os.clock()) ~= "number" then
  error("os.clock type mismatch")
end

local utc = os.date("!%Y-%m-%d %H:%M:%S", 0)
local parts = os.date("!*t", 0)
if utc ~= "1970-01-01 00:00:00" or parts.year ~= 1970 or parts.month ~= 1 or parts.day ~= 1 or parts.wday ~= 5 or parts.yday ~= 1 then
  error("os.date fixed timestamp mismatch")
end

local stamp = 946684800
local roundtrip = os.time(os.date("*t", stamp))
if roundtrip ~= stamp then
  error("os.time table roundtrip mismatch")
end
