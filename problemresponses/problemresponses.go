package problemresponses

// -------------- Structs --------------
// ProblemResponse -- Defined by https://www.rfc-editor.org/rfc/rfc9457.html#section-3
type ProblemResponse struct {
	Type     string `json:"type" xml:"type"`
	Title    string `json:"title" xml:"title"`
	Detail   string `json:"detail" xml:"detail"`
	Instance string `json:"instance" xml:"instance"`
}

// NewProblemResponse -- Create a new ProblemResponse
func NewProblemResponse(Type, Title, Detail, Instance string) *ProblemResponse {
	return &ProblemResponse{
		Type:     Type,
		Title:    Title,
		Detail:   Detail,
		Instance: Instance,
	}
}
