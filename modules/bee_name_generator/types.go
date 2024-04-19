package beenamegenerator

// NameResponse struct
type NameResponse struct {
	Name string `json:"name" xml:"name"`
}

// NewNameResponse - Create a new NameResponse
func NewNameResponse(name string) NameResponse {
	return NameResponse{
		Name: name,
	}
}
