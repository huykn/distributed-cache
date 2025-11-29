abc = {"test-1763879237", "test-1763879238", "test-1763879239", "test-1763879240", "test-1763879241"}
local str = ""

function url_encode(str)
    if (str) then
        str = string.gsub(str, "\n", "\r\n")
        str = string.gsub(str, "([^%w ])",
            function (c) return string.format("%%%02X", string.byte(c)) end)
        str = string.gsub(str, " ", "+")
    end
    return str    
end

function GeneratedString() 
    str = ""
    for i=1,1 do
        rdm = math.random(1,5)
        str = str..abc[rdm]
    end
    return str
end

function request()
    local base_url = 'http://127.0.0.1:9080/post?id='
    local random_string = GeneratedString()
    local encoded_string = url_encode(random_string)
    local random_url = base_url .. encoded_string
    return wrk.format("GET", random_url)
end