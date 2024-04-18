package mcstatus

import (
	"image/png"
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	service := NewService()
	mux.HandleFunc("/api/v1/mcstatus/{host}", GetServerStatus(service))
	mux.HandleFunc("/api/v1/mcstatus/icon/{host}", GetIcon(service))
	return mux
}

// Route that returns the server status
func GetServerStatus(s *service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.PathValue("host")
		isBedrock := r.URL.Query().Get("bedrock") == "true"
		queryEnabled := r.URL.Query().Get("query") == "true"
		raw := r.URL.Query().Get("raw") == "true"
		port, err := strconv.Atoi(r.URL.Query().Get("port"))
		if err != nil {
			if isBedrock {
				port = 19132
			} else {
				port = 25565
			}
		}

		status, err := s.GetServerStatus(host, port, isBedrock, queryEnabled)
		if err != nil {
			responses.SendAndEncodeInternalServerError(w, r, err.Error())
			return
		}
		if !raw {
			status.Raw = nil
		}

		responses.SendAndEncodeStruct(w, r, http.StatusOK, status)
	}
}

// Route that returns the server icon as a PNG (base64 encoded string didn't work for some reason)
func GetIcon(s *service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.PathValue("host")
		isBedrock := r.URL.Query().Get("bedrock") == "true"
		queryEnabled := r.URL.Query().Get("query") == "true"
		port, err := strconv.Atoi(r.URL.Query().Get("port"))
		if err != nil {
			if isBedrock {
				port = 19132
			} else {
				port = 25565
			}
		}

		status, err := s.GetServerStatus(host, port, isBedrock, queryEnabled)
		if err != nil {
			responses.SendAndEncodeInternalServerError(w, r, err.Error())
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		png.Encode(w, status.Icon)
	}
}
