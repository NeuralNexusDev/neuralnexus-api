package switchboard

import "github.com/goccy/go-json"

type Message struct {
	Version     int    `json:"version"`
	Origin      string `json:"origin"`
	Dest        string `json:"dest"`
	MessageID   string `json:"message_id"`
	MessageType string `json:"message_type,omitempty"`
	Encrypted   bool   `json:"encrypted,omitempty"`
	EncScheme   string `json:"enc_scheme,omitempty"`
	Content     string `json:"content"`
}

type Relay struct {
	Sources map[string]Source   `json:"sources"`
	Routes  map[string][]string `json:"routes"`
}

type Source struct {
	Protocol string `json:"protocol"`
	Platform string `json:"platform"`
}

func (s *Source) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, s)
	if err != nil {
		return err
	}
	if s.Protocol == "" {
		s.Protocol = "http+json"
	}
	return nil
}
