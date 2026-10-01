package mcstatus

import (
	"errors"
	"image"
	"image/png"
	"log"
	"net"
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
)

const (
	msgJavaStatusFailed        = "failed to get java server status"
	msgBedrockStatusFailed     = "failed to get bedrock server status"
	msgFailedToGetServerStatus = "Failed to get server status"
	msgIconUnavailable         = "Failed to load server icon"
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

func respondStatusFailure(w http.ResponseWriter, r *http.Request, err error) {
	log.Println("[Error]: Unable to get server status:\n\t", err)
	for _, m := range statusFailures {
		if errors.Is(err, m.err) {
			responses.NotFound(w, r, m.msg)
			return
		}
	}
	responses.InternalServerError(w, r, msgFailedToGetServerStatus)
}

func splitHostPort(address string, isBedrock bool) (string, int) {
	defaultPort := defaultJavaPort
	if isBedrock {
		defaultPort = defaultBedrockPort
	}
	if strings.HasPrefix(address, "[") {
		if host, rawPort, err := net.SplitHostPort(address); err == nil {
			if port, err := strconv.Atoi(rawPort); err == nil {
				return host, port
			}
		} else if strings.HasSuffix(address, "]") {
			return strings.Trim(address, "[]"), defaultPort
		}
		return address, defaultPort
	}
	if ip, err := netip.ParseAddr(address); err == nil && ip.Is6() {
		return address, defaultPort
	}
	if i := strings.LastIndex(address, ":"); i >= 0 {
		if port, err := strconv.Atoi(address[i+1:]); err == nil {
			return address[:i], port
		}
	}
	return address, defaultPort
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
		host, port := splitHostPort(host, isBedrock)
		queryPort := queryPortOrDefault(r.URL.Query().Get("query_port"), port)

		status, err := s.GetServerStatus(host, port, isBedrock, queryEnabled, queryPort)
		if err != nil {
			respondStatusFailure(w, r, err)
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
		if isBedrock {
			writeStockIcon(w, r, bedrockIconFile)
			return
		}
		host, port := splitHostPort(host, false)

		status, err := s.GetJavaServerStatus(host, port, false, 0)
		if errors.Is(err, ErrJavaStatus) {
			writeStockIcon(w, r, defaultIconFile)
			return
		}
		if err != nil {
			respondStatusFailure(w, r, err)
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
		host, port := splitHostPort(host, isBedrock)
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
