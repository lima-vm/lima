#!/usr/libexec/flua
local yaml = require("lyaml")
local fpath = "/mnt/lima-cidata/user-data"
local f = io.open(fpath, "r")
if not f then
	error("Could not open " .. fpath)
end
local content = f:read("*a")
f:close()
local config = yaml.load(content)
for i, row in ipairs(config.mounts) do
	for j, value in ipairs(row) do
		io.write(value, "\t")
	end
	io.write("\n")
end
