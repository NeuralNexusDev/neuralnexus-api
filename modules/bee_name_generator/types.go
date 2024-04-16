package bee_name_generator

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

// AmountResponse struct
type AmountResponse struct {
	Amount int64 `json:"amount" xml:"amount"`
}

// NewAmountResponse - Create a new AmountResponse
func NewAmountResponse(amount int64) AmountResponse {
	return AmountResponse{
		Amount: amount,
	}
}

// SuggestionsResponse struct
type SuggestionsResponse struct {
	Suggestions []string `json:"suggestions" xml:"suggestions"`
}

// NewSuggestionsResponse - Create a new SuggestionsResponse
func NewSuggestionsResponse(suggestions []string) SuggestionsResponse {
	return SuggestionsResponse{
		Suggestions: suggestions,
	}
}
