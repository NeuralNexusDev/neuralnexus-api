package mcstatus

import (
	"image/png"
	"net/http"
	"strconv"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	service := NewService()
	mux.HandleFunc("/api/v1/mcstatus/{host}", GetServerStatus(service))
	mux.HandleFunc("/api/v1/mcstatus/icon/{host}", GetIcon(service))
	return mux
}

// GetServerStatus - Route that returns the server status
func GetServerStatus(s MCStatusService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.PathValue("host")
		isBedrock := r.URL.Query().Get("bedrock") == "true"
		queryEnabled := r.URL.Query().Get("query") == "true"
		raw := r.URL.Query().Get("raw") == "true"
		port, err := strconv.Atoi(host[strings.LastIndex(host, ":")+1:])
		if err != nil {
			if isBedrock {
				port = 19132
			} else {
				port = 25565
			}
		}
		queryPort, err := strconv.Atoi(r.URL.Query().Get("query_port"))
		if err != nil {
			queryPort = port
		}

		status, err := s.GetServerStatus(host, port, isBedrock, queryEnabled, queryPort)
		if err != nil {
			responses.SendAndEncodeNotFound(w, r, err.Error())
			return
		}
		if !raw {
			status.Raw = nil
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, status)
	}
}

// GetIcon - Route that returns the server icon as a PNG
func GetIcon(s MCStatusService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.PathValue("host")
		isBedrock := r.URL.Query().Get("bedrock") == "true"
		if isBedrock {
			responses.SendAndEncodeBadRequest(w, r, "Bedrock servers do not have icons.")
		}
		port, err := strconv.Atoi(host[strings.LastIndex(host, ":")+1:])
		if err != nil {
			port = 25565
		}

		status, err := s.GetJavaServerStatus(host, port, false, 0)
		if err != nil {
			responses.SendAndEncodeNotFound(w, r, err.Error())
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		png.Encode(w, status.Icon)
	}
}
