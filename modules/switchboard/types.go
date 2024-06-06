package switchboard

type Header struct {
	Version int    `json:"version"`
	Origin  string `json:"origin"`
	Dest    string `json:"dest"`
}

type Body struct {
	MessageID string `json:"message_id"`
	Encrypted bool   `json:"encrypted,omitempty"`
	EncScheme string `json:"enc_scheme,omitempty"`
	Content   string `json:"content"`
}

type Packet struct {
	Header Header `json:"header"`
	Body   Body   `json:"body"`
}
