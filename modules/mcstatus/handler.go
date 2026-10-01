package mcstatus

import (
	"errors"
	"image/png"
	"log"
	"net/http"
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
	msgBedrockNoIcons          = "Bedrock servers do not have icons."
	msgServerNoIcon            = "Server has no icon."
)

type failureMapping struct {
	err     error
	respond func(http.ResponseWriter, *http.Request, string)
	msg     string
}

var statusFailures = []failureMapping{
	{ErrJavaStatus, responses.BadGateway, msgJavaStatusFailed},
	{ErrBedrockStatus, responses.BadGateway, msgBedrockStatusFailed},
}

func respondStatusFailure(w http.ResponseWriter, r *http.Request, err error) {
	log.Println("[Error]: Unable to get server status:\n\t", err)
	for _, m := range statusFailures {
		if errors.Is(err, m.err) {
			m.respond(w, r, m.msg)
			return
		}
	}
	responses.InternalServerError(w, r, msgFailedToGetServerStatus)
}

func splitHostPort(address string, isBedrock bool) (string, int) {
	if i := strings.LastIndex(address, ":"); i >= 0 {
		if port, err := strconv.Atoi(address[i+1:]); err == nil {
			return address[:i], port
		}
	}
	if isBedrock {
		return address, defaultBedrockPort
	}
	return address, defaultJavaPort
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

// IconHandler - Route that returns the server icon as a PNG
func IconHandler(s MCStatusService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.PathValue("host")
		isBedrock := r.URL.Query().Get("bedrock") == "true"
		if isBedrock {
			responses.BadRequest(w, r, msgBedrockNoIcons)
			return
		}
		host, port := splitHostPort(host, false)

		status, err := s.GetJavaServerStatus(host, port, false, 0)
		if err != nil {
			respondStatusFailure(w, r, err)
			return
		}
		if status.Icon == nil {
			responses.NotFound(w, r, msgServerNoIcon)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		png.Encode(w, status.Icon)
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
