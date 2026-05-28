#!/usr/bin/env lua
if "shebang" .. ":" .. (6 * 7) ~= "shebang:42" then
  error("shebang script mismatch")
end
