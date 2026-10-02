package mcstatus

import (
	"errors"
	"image"
	"image/png"
	"log"
	"net/http"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	minPort            = 1
	maxPort            = 65535
	defaultJavaPort    = 25565
	defaultBedrockPort = 19132
	maxHostLength      = 253
	maxLabelLength     = 63
)

const (
	msgJavaStatusFailed        = "failed to get java server status"
	msgBedrockStatusFailed     = "failed to get bedrock server status"
	msgFailedToGetServerStatus = "Failed to get server status"
	msgIconUnavailable         = "Failed to load server icon"
	msgInvalidHost             = "The host must be a domain name, an IPv4 address or an IPv6 address, optionally followed by a port."
)

const (
	bedrockIconFile = "bedrock.png"
	defaultIconFile = "default.png"
	legacyIconFile  = "legacy.png"
)

var iconDir = filepath.Join("public", "mcstatus", "icons")

type failureMapping struct {
	err error
	msg string
}

var statusFailures = []failureMapping{
	{ErrJavaStatus, msgJavaStatusFailed},
	{ErrBedrockStatus, msgBedrockStatusFailed},
}

type offlineProblem struct {
	*responses.Problem
	Host string `json:"host" xml:"host"`
	Port int    `json:"port" xml:"port"`
}

func respondStatusFailure(w http.ResponseWriter, r *http.Request, err error, host string, port int) {
	log.Println("[Error]: Unable to get server status:\n\t", err)
	for _, m := range statusFailures {
		if errors.Is(err, m.err) {
			responses.SendProblemStruct(w, r, http.StatusNotFound, offlineProblem{responses.NewNotFoundProblem(m.msg), host, port})
			return
		}
	}
	responses.InternalServerError(w, r, msgFailedToGetServerStatus)
}

func parsePort(raw string) (int, bool) {
	if len(raw) == 0 || len(raw) > 5 {
		return 0, false
	}
	port := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return 0, false
		}
		port = port*10 + int(raw[i]-'0')
	}
	return port, port >= minPort && port <= maxPort
}

func parseIPv6Host(raw string) (string, bool) {
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') && c != ':' && c != '.' {
			return "", false
		}
	}
	ip, err := netip.ParseAddr(raw)
	if err != nil || !ip.Is6() {
		return "", false
	}
	return ip.String(), true
}

func parseHost(address string, isBedrock bool) (string, int, bool) {
	port := defaultJavaPort
	if isBedrock {
		port = defaultBedrockPort
	}
	if address == "" {
		return "", 0, false
	}

	if address[0] == '[' {
		end := strings.IndexByte(address, ']')
		if end < 0 {
			return "", 0, false
		}
		host, ok := parseIPv6Host(address[1:end])
		if !ok {
			return "", 0, false
		}
		if rest := address[end+1:]; rest != "" {
			if rest[0] != ':' {
				return "", 0, false
			}
			if port, ok = parsePort(rest[1:]); !ok {
				return "", 0, false
			}
		}
		return host, port, true
	}

	colons := strings.Count(address, ":")
	if colons >= 2 {
		host, ok := parseIPv6Host(address)
		return host, port, ok
	}

	name := address
	if colons == 1 {
		i := strings.IndexByte(address, ':')
		name = address[:i]
		var ok bool
		if port, ok = parsePort(address[i+1:]); !ok {
			return "", 0, false
		}
	}
	if len(name) == 0 || len(name) > maxHostLength {
		return "", 0, false
	}
	dots := 0
	upper := false
	labelStart := 0
	lastLabelNumeric := true
	for i := 0; i <= len(name); i++ {
		if i == len(name) || name[i] == '.' {
			if i-labelStart < 1 || i-labelStart > maxLabelLength || name[labelStart] == '-' || name[i-1] == '-' {
				return "", 0, false
			}
			if i < len(name) {
				dots++
				labelStart = i + 1
				lastLabelNumeric = true
			}
			continue
		}
		switch c := name[i]; {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'z' || c == '-' || c == '_':
			lastLabelNumeric = false
		case c >= 'A' && c <= 'Z':
			upper = true
			lastLabelNumeric = false
		default:
			return "", 0, false
		}
	}
	if dots == 0 {
		return "", 0, false
	}
	if lastLabelNumeric {
		ip, err := netip.ParseAddr(name)
		if err != nil || !ip.Is4() {
			return "", 0, false
		}
		return ip.String(), port, true
	}
	if upper {
		name = strings.ToLower(name)
	}
	return name, port, true
}

func queryPortOrDefault(raw string, port int) int {
	queryPort, err := strconv.Atoi(raw)
	if err != nil || queryPort < minPort || queryPort > maxPort {
		return port
	}
	return queryPort
}

// ServerStatusHandler - Route that returns the server status
func ServerStatusHandler(s MCStatusService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.PathValue("host")
		isBedrock := r.URL.Query().Get("bedrock") == "true"
		queryEnabled := r.URL.Query().Get("query") == "true"
		raw := r.URL.Query().Get("raw") == "true"
		host, port, ok := parseHost(host, isBedrock)
		if !ok {
			responses.BadRequest(w, r, msgInvalidHost)
			return
		}
		queryPort := queryPortOrDefault(r.URL.Query().Get("query_port"), port)

		status, err := s.GetServerStatus(host, port, isBedrock, queryEnabled, queryPort)
		if err != nil {
			respondStatusFailure(w, r, err, host, port)
			return
		}
		if !raw {
			status.Raw = nil
		}
		responses.StructOK(w, r, status)
	}
}

func writeIcon(w http.ResponseWriter, img image.Image) {
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	png.Encode(w, img)
}

func writeStockIcon(w http.ResponseWriter, r *http.Request, name string) {
	img, err := LoadImgFromFile(filepath.Join(iconDir, name))
	if err != nil {
		log.Println("[Error]: Unable to load stock icon:\n\t", err)
		responses.InternalServerError(w, r, msgIconUnavailable)
		return
	}
	writeIcon(w, img)
}

// IconHandler - Route that returns the server icon as a PNG
func IconHandler(s MCStatusService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.PathValue("host")
		isBedrock := r.URL.Query().Get("bedrock") == "true"
		host, port, ok := parseHost(host, isBedrock)
		if !ok {
			responses.BadRequest(w, r, msgInvalidHost)
			return
		}
		if isBedrock {
			writeStockIcon(w, r, bedrockIconFile)
			return
		}

		status, err := s.GetJavaServerStatus(host, port, false, 0)
		if errors.Is(err, ErrJavaStatus) {
			writeStockIcon(w, r, defaultIconFile)
			return
		}
		if err != nil {
			respondStatusFailure(w, r, err, host, port)
			return
		}
		if status.Icon == nil {
			name := defaultIconFile
			if status.Legacy {
				name = legacyIconFile
			}
			writeStockIcon(w, r, name)
			return
		}

		writeIcon(w, status.Icon)
	}
}

// SimpleStatusHandler - Route that returns the server status in a simple format
func SimpleStatusHandler(s MCStatusService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.PathValue("host")
		isBedrock := r.URL.Query().Get("bedrock") == "true"
		queryEnabled := r.URL.Query().Get("query") == "true"
		host, port, ok := parseHost(host, isBedrock)
		if !ok {
			responses.BadRequest(w, r, msgInvalidHost)
			return
		}
		queryPort := queryPortOrDefault(r.URL.Query().Get("query_port"), port)

		status := "Online"
		statusCode := http.StatusOK
		_, err := s.GetServerStatus(host, port, isBedrock, queryEnabled, queryPort)
		if err != nil {
			status = "Offline"
			statusCode = http.StatusNotFound
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(statusCode)
		w.Write([]byte(status))
	}
}
