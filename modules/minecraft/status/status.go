package status

type Version struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type PlayerSample struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type Players struct {
	Max    int            `json:"max"`
	Online int            `json:"online"`
	Sample []PlayerSample `json:"sample"` // Optional, some servers include additional information
}

type StatusResponse struct {
	Version            Version   `json:"version"`
	Players            Players   `json:"players"`            // Optional
	Description        Component `json:"description"`        // Optional
	Favicon            string    `json:"favicon"`            // Optional
	EnforcesSecureChat *bool     `json:"enforcesSecureChat"` // version-specific
}
