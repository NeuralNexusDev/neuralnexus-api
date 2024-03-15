URL = "https://api.neuralnexus.dev/api/v1/cct-turtle/startup.lua"
local ok, err = http.checkURL(URL)
if not ok then
    print("Failed to check URL: " .. err)
    os.sleep(5)
    os.reboot()
end

local response = http.get(URL)
if not response then
    print("Failed to fetch URL")
    os.sleep(5)
    os.reboot()
end

local data = response.readAll()
response.close()

local file = fs.open("run.lua", "wb")
file.write(data)
file.close()

shell.run("run.lua")
