package mcstatus

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"strings"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	service := NewService()
	mux.HandleFunc("/api/v1/mcstatus/{address}", GetServerStatus(service))
	mux.HandleFunc("/api/v1/mcstatus/icon/{address}", GetIcon(service))
	return mux
}

// Route that returns the server status
func GetServerStatus(s *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the query params
		serverInfo := getServerInfo(r)

		// Disable query if there is no accept header (assume embed)
		if strings.Split(r.Header.Get("Accept"), ",")[0] == "" {
			serverInfo.EnableQuery = false
		}

		var status int = http.StatusOK

		// Get the server status
		resp, _, err := s.ServerStatus(serverInfo)
		if err != nil {
			status = http.StatusNotFound
		}

		// Check the request type
		if strings.Split(r.Header.Get("Content-Type"), ",")[0] == "application/json" {
			// Serve the json
			w.Header().Set("Content-Type", "application/json")
			if resp.Name == "Server Offline" {
				status = http.StatusNotFound
			}
			json.NewEncoder(w).Encode(resp)
			return
		} else if strings.Split(r.Header.Get("Accept"), ",")[0] == "text/html" {
			html, err := os.ReadFile("public/mcstatus/templates/status.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			// Replace the placeholders
			htmlString := string(html)
			htmlString = strings.ReplaceAll(htmlString, "{{SERVER_URL}}", SERVER_URL)
			htmlString = strings.ReplaceAll(htmlString, "{{ADDRESS}}", resp.Connect)
			htmlString = strings.ReplaceAll(htmlString, "{{MOTD}}", resp.Name)
			htmlString = strings.ReplaceAll(htmlString, "{{FAVICON}}", resp.Favicon)
			htmlString = strings.ReplaceAll(htmlString, "{{ONLINE_PLAYERS}}", fmt.Sprint(resp.OnlinePlayers))
			htmlString = strings.ReplaceAll(htmlString, "{{MAX_PLAYERS}}", fmt.Sprint(resp.MaxPlayers))

			// Serve the html
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(htmlString))
			return
		} else {
			html, err := os.ReadFile("public/mcstatus/templates/embed.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			// Build response address string
			var queryParams []string = []string{}
			if serverInfo.IsBedrock {
				queryParams = append(queryParams, "is_bedrock=true")
			}
			if serverInfo.Port != 25565 {
				queryParams = append(queryParams, "port="+fmt.Sprint(serverInfo.Port))
			}
			if serverInfo.QueryPort != serverInfo.Port {
				queryParams = append(queryParams, "query_port="+fmt.Sprint(serverInfo.QueryPort))
			}
			if !serverInfo.EnableQuery {
				queryParams = append(queryParams, "enable_query=false")
			}

			// Format the query params
			var queryParamsString string = ""
			if len(queryParams) > 0 {
				queryParamsString = "?" + strings.Join(queryParams, "&")
			}

			// Replace the placeholders
			htmlString := string(html)
			htmlString = strings.ReplaceAll(htmlString, "{{SERVER_URL}}", SERVER_URL)
			htmlString = strings.ReplaceAll(htmlString, "{{ADDRESS}}", resp.Connect)
			htmlString = strings.ReplaceAll(htmlString, "{{ONLINE_PLAYERS}}", fmt.Sprint(resp.OnlinePlayers))
			htmlString = strings.ReplaceAll(htmlString, "{{MAX_PLAYERS}}", fmt.Sprint(resp.MaxPlayers))
			htmlString = strings.ReplaceAll(htmlString, "{{MOTD_TEXT1}}", strings.Split(resp.Name, "\\n")[0])
			var motd2String string = ""
			if len(strings.Split(resp.Name, "\\n")) > 1 {
				motd2String = strings.Split(resp.Name, "\\n")[1]
			}
			htmlString = strings.ReplaceAll(htmlString, "{{MOTD_TEXT2}}", motd2String)
			htmlString = strings.ReplaceAll(htmlString, "{{VERSION}}", resp.Version)
			htmlString = strings.ReplaceAll(htmlString, "{{ADDRESS_STR}}", serverInfo.Address+queryParamsString)
			htmlString = strings.ReplaceAll(htmlString, "{{FAVICON}}", resp.Favicon)

			// Serve the html
			w.WriteHeader(status)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(htmlString))
		}
	}
}

// Route that returns the server icon as a PNG (base64 encoded string didn't work for some reason)
func GetIcon(s *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the query params
		serverInfo := getServerInfo(r)
		serverInfo.EnableQuery = false

		var icon image.Image
		var status int = http.StatusOK

		// Get the server status
		if serverInfo.IsBedrock {
			// Get Bedrock icon
			icon = bedrockIcon
			status = http.StatusNoContent
		} else {
			// Get Java server status
			_, image, err := serverInfo.JavaServerStatus()

			if err == nil {
				// Set the icon as the server icon
				icon = image
			} else {
				// Set the icon as the default icon
				icon = defaultIcon
				status = http.StatusNotFound
			}
		}

		// Set the content type as image/png
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(status)
		png.Encode(w, icon)
	}
}
