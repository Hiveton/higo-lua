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

local date_text = os.date("!%Y-%m-%d %H:%M:%S", 0)
local date_table = os.date("!*t", 0)
if date_text ~= "1970-01-01 00:00:00" or date_table.year ~= 1970 or date_table.month ~= 1 or date_table.day ~= 1 or date_table.hour ~= 0 or date_table.min ~= 0 or date_table.sec ~= 0 or date_table.isdst ~= false then
  error("os.date explicit time mismatch")
end
