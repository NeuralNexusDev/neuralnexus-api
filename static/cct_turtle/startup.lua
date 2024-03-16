-------------------------------------------------------------------------- JSON API
--- Credit: http://www.computercraft.info/forums2/index.php?/user/562-elvishjerricco/, http://pastebin.com/4nRg9CHU, http://www.computercraft.info/forums2/index.php?/topic/5854-json-api-v201-for-computercraft/
------------------------------------------------------------------ utils
local controls = {["\n"]="\\n", ["\r"]="\\r", ["\t"]="\\t", ["\b"]="\\b", ["\f"]="\\f", ["\""]="\\\"", ["\\"]="\\\\"}

local function isArray(t)
	local max = 0
	for k,v in pairs(t) do
		if type(k) ~= "number" then
			return false
		elseif k > max then
			max = k
		end
	end
	return max == #t
end

local whites = {['\n']=true; ['\r']=true; ['\t']=true; [' ']=true; [',']=true; [':']=true}
function removeWhite(str)
	while whites[str:sub(1, 1)] do
		str = str:sub(2)
	end
	return str
end

------------------------------------------------------------------ encoding

local function encodeCommon(val, pretty, tabLevel, tTracking)
	local str = ""

	-- Tabbing util
	local function tab(s)
		str = str .. ("\t"):rep(tabLevel) .. s
	end

	local function arrEncoding(val, bracket, closeBracket, iterator, loopFunc)
		str = str .. bracket
		if pretty then
			str = str .. "\n"
			tabLevel = tabLevel + 1
		end
		for k,v in iterator(val) do
			tab("")
			loopFunc(k,v)
			str = str .. ","
			if pretty then str = str .. "\n" end
		end
		if pretty then
			tabLevel = tabLevel - 1
		end
		if str:sub(-2) == ",\n" then
			str = str:sub(1, -3) .. "\n"
		elseif str:sub(-1) == "," then
			str = str:sub(1, -2)
		end
		tab(closeBracket)
	end

	-- Table encoding
	if type(val) == "table" then
		assert(not tTracking[val], "Cannot encode a table holding itself recursively")
		tTracking[val] = true
		if isArray(val) then
			arrEncoding(val, "[", "]", ipairs, function(k,v)
				str = str .. encodeCommon(v, pretty, tabLevel, tTracking)
			end)
		else
			arrEncoding(val, "{", "}", pairs, function(k,v)
				assert(type(k) == "string", "JSON object keys must be strings", 2)
				str = str .. encodeCommon(k, pretty, tabLevel, tTracking)
				str = str .. (pretty and ": " or ":") .. encodeCommon(v, pretty, tabLevel, tTracking)
			end)
		end
	-- String encoding
	elseif type(val) == "string" then
		str = '"' .. val:gsub("[%c\"\\]", controls) .. '"'
	-- Number encoding
	elseif type(val) == "number" or type(val) == "boolean" then
		str = tostring(val)
	else
		error("JSON only supports arrays, objects, numbers, booleans, and strings", 2)
	end
	return str
end

function encode(val)
	return encodeCommon(val, false, 0, {})
end

function encodePretty(val)
	return encodeCommon(val, true, 0, {})
end

------------------------------------------------------------------ decoding

local decodeControls = {}
for k,v in pairs(controls) do
	decodeControls[v] = k
end

function parseBoolean(str)
	if str:sub(1, 4) == "true" then
		return true, removeWhite(str:sub(5))
	else
		return false, removeWhite(str:sub(6))
	end
end

function parseNull(str)
	return nil, removeWhite(str:sub(5))
end

local numChars = {['e']=true; ['E']=true; ['+']=true; ['-']=true; ['.']=true}
function parseNumber(str)
	local i = 1
	while numChars[str:sub(i, i)] or tonumber(str:sub(i, i)) do
		i = i + 1
	end
	local val = tonumber(str:sub(1, i - 1))
	str = removeWhite(str:sub(i))
	return val, str
end

function parseString(str)
	str = str:sub(2)
	local s = ""
	while str:sub(1,1) ~= "\"" do
		local next = str:sub(1,1)
		str = str:sub(2)
		assert(next ~= "\n", "Unclosed string")

		if next == "\\" then
			local escape = str:sub(1,1)
			str = str:sub(2)

			next = assert(decodeControls[next..escape], "Invalid escape character")
		end

		s = s .. next
	end
	return s, removeWhite(str:sub(2))
end

function parseArray(str)
	str = removeWhite(str:sub(2))

	local val = {}
	local i = 1
	while str:sub(1, 1) ~= "]" do
		local v = nil
		v, str = parseValue(str)
		val[i] = v
		i = i + 1
		str = removeWhite(str)
	end
	str = removeWhite(str:sub(2))
	return val, str
end

function parseObject(str)
	str = removeWhite(str:sub(2))

	local val = {}
	while str:sub(1, 1) ~= "}" do
		local k, v = nil, nil
		k, v, str = parseMember(str)
		val[k] = v
		str = removeWhite(str)
	end
	str = removeWhite(str:sub(2))
	return val, str
end

function parseMember(str)
	local k = nil
	k, str = parseValue(str)
	local val = nil
	val, str = parseValue(str)
	return k, val, str
end

function parseValue(str)
	local fchar = str:sub(1, 1)
	if fchar == "{" then
		return parseObject(str)
	elseif fchar == "[" then
		return parseArray(str)
	elseif tonumber(fchar) ~= nil or numChars[fchar] then
		return parseNumber(str)
	elseif str:sub(1, 4) == "true" or str:sub(1, 5) == "false" then
		return parseBoolean(str)
	elseif fchar == "\"" then
		return parseString(str)
	elseif str:sub(1, 4) == "null" then
		return parseNull(str)
	end
	return nil
end

function decode(str)
	str = removeWhite(str)
	t = parseValue(str)
	return t
end

function decodeFromFile(path)
	local file = assert(fs.open(path, "r"))
	local decoded = decode(file.readAll())
	file.close()
	return decoded
end


--------------------------------------------------------------------------  Start of Turtle Bot
--------- Utils ---------
local function rename()
    if os.getComputerLabel() == nil then
        local str=""
        --all = {"0","1","2","3","4","5","6","7","8","9","a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x","y","z"}
        local all = {"a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x","y","z"}
        for i=1, 10 do str = str..all[math.random(1, #all)] end
        os.setComputerLabel(str)
    end
end

local function initNewTurtle()
    turtle.select(1)
    turtle.place()
    turtle.select(2)
    turtle.drop(1)
	turtle.select(3)
	turtle.drop(1)
    peripheral.call("front","turnOn")
end

local function get_facing(x1,z1,x2,z2)
    --    north = 0 = z = -1
    --    east = 270 = x = 1
    --    south = 180 = z = 1
    --    west = 90 = x = -1
    local x_diff = x2-x1
    local z_diff = z2-z1
    if x_diff == -1 then
        return 90
    end
    if x_diff == 1 then
        return 270
    end
    if z_diff == -1 then
        return 0
    end
    if z_diff == 1 then
        return 180
    end
end

local function update_rotation(func_string)
    if string.find(func_string, "Left") then
        if Facing == 0 then
            Facing = 90
        elseif Facing == 90 then
            Facing = 180
        elseif Facing == 180 then
            Facing = 270
        elseif Facing == 270 then
            Facing = 0
        end
    elseif string.find(func_string, "Right") then
        if Facing == 0 then
            Facing = 270
        elseif Facing == 90 then
            Facing = 0
        elseif Facing == 180 then
            Facing = 90
        elseif Facing == 270 then
            Facing = 180
        end
    end
end

local function get_turtle()
    -- Current xyz pos
    local x,z,y = gps.locate(5)

    -- formatted json string output
    return "\"turtle\":{\"label\":\""..os.getComputerLabel().."\",\"id\":"..tostring(os.getComputerID())..",\"fuel\":\""..tostring(turtle.getFuelLevel()).."\",\"position\":\""..tostring(x)..","..tostring(y)..","..tostring(z).."\",\"facing\":\""..tostring(Facing).."\"}"
end

local function observe()
    -- observe the world
    local if_up, up = turtle.inspectUp() 
    local if_front, front = turtle.inspect()
    local if_down, down = turtle.inspectDown()

    -- formatted json string output
    return "\"blocks\":{\"up\":{\"name\":\""..tostring(up.name).."\",\"metadata\":\""..tostring(up.metadata).."\",\"state\":\""..tostring(up.state).."\"},\"front\":{\"name\":\""..tostring(front.name).."\",\"metadata\":\""..tostring(front.metadata).."\",\"state\":\""..tostring(up.state).."\"},\"down\":{\"name\":\""..tostring(down.name).."\",\"metadata\":\""..tostring(down.metadata).."\",\"state\":\""..tostring(up.state).."\"}}"
end

local function get_inv()
	local inv = "\"inventory\":["
	for i = 1,15 do
		local item = turtle.getItemDetail(i)
		if (item==nil) then
			inv = inv.."{\"name\":\""..tostring(nil).."\",\"damage\":\""..tostring(nil).."\",\"count\":\""..tostring(0).."\"},"
		else
			inv = inv.."{\"name\":\""..tostring(item.name).."\",\"damage\":\""..tostring(item.damage).."\",\"count\":\""..tostring(item.count).."\"},"
		end
	end
	local item = turtle.getItemDetail(16)
	if (item==nil) then
		inv = inv.."{\"name\":\""..tostring(nil).."\",\"damage\":\""..tostring(nil).."\",\"count\":\""..tostring(0).."\"}"
	else
		inv = inv.."{\"name\":\""..tostring(item.name).."\",\"damage\":\""..tostring(item.damage).."\",\"count\":\""..tostring(item.count).."\"}"
	end
	return inv.."]"
end

local function if_get_inv(func_string)
	if string.find(func_string, "dig") or string.find(func_string, "Dig") or string.find(func_string, "suck") or string.find(func_string, "Suck") or string.find(func_string, "drop") or string.find(func_string, "Drop") or string.find(func_string, "place") or string.find(func_string, "Place") then
        return ","..get_inv()
    else
        return ""
    end
end

local function dumpInventory()
	if turtle.detectDown() then
        turtle.digDown()
    end
	turtle.select(1)
	turtle.placeDown()

	local inv = "\"payload\":["
	for i = 1,15 do
		local item = turtle.getItemDetail(i)
		if (item==nil) then
			inv = inv.."{\"name\":\""..tostring(nil).."\",\"damage\":\""..tostring(nil).."\",\"count\":\""..tostring(0).."\"},"
		else
			inv = inv.."{\"name\":\""..tostring(item.name).."\",\"damage\":\""..tostring(item.damage).."\",\"count\":\""..tostring(item.count).."\"},"
		end
		turtle.select(i)
		turtle.dropDown()
	end
	local item = turtle.getItemDetail(16)
	if (item==nil) then
		inv = inv.."{\"name\":\""..tostring(nil).."\",\"damage\":\""..tostring(nil).."\",\"count\":\""..tostring(0).."\"}"
	else
		inv = inv.."{\"name\":\""..tostring(item.name).."\",\"damage\":\""..tostring(item.damage).."\",\"count\":\""..tostring(item.count).."\"}"
	end
	turtle.select(16)
	turtle.dropDown()

	turtle.select(1)
	turtle.digDown()
	turtle.select(2)

	return inv.."]"
end

--------- Actions ---------
local function digMoveForward()
    turtle.dig()
    turtle.forward()
end

local function digMoveUp()
    turtle.digUp()
    turtle.up()
end

local function digMoveDown()
    turtle.digDown()
    turtle.down()
end

--------- Start of Program ---------

Facing = 0
local function main()
	rename()

	local x1, _ ,z1 = gps.locate(5)
	if not x1 then
		print("No GPS Signal")
		return
	end

	digMoveForward()
	local x2, _ ,z2 = gps.locate(5)
	if not x2 then
		print("No GPS Signal")
		return
	end

	turtle.back()
	Facing = get_facing(x1, z1, x2, z2)

	local label = os.getComputerLabel()

	local ws, err = http.websocket("wss://api.neuralnexus.dev/ws/v1/cct-turtle/"..label)
	if ws then
		print("> Connected")
		ws.send("{"..get_turtle()..","..observe()..","..get_inv().."}")
		while true do
			local ok, msg = pcall(ws.receive)
			if not ok then
				print("Error: > Disconnected")
				break
			end
			local response = "{"
			print(msg)
			local ok, obj = pcall(decode, msg)
			if not ok then
				if not msg then
					print("Error: nil message")
				else 
					print("Error: "..msg)
				end
			elseif obj and obj["func"] then
				if string.find(obj["func"], "digMoveForward") then
					digMoveForward()
				elseif string.find(obj["func"], "digMoveUp") then
					digMoveUp()
				elseif string.find(obj["func"], "digMoveDown") then
					digMoveDown()
				elseif string.find(obj["func"], "dumpInventory") then
					response = response..dumpInventory()..","
				elseif string.find(obj["func"], "initNewTurtle") then
					initNewTurtle()
				else
					local func = loadstring(obj["func"])
					if func then
						func()
					end
				end
				update_rotation(obj["func"])
				ws.send(response..get_turtle()..","..observe()..","..get_inv().."}")
			end
		end
	end
end

main()
os.sleep(5)
os.reboot()
