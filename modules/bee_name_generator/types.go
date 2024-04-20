package beenamegenerator

import (
	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/bngpb"
)

// NewBeeName - Create a new BeeName
func NewBeeName(name string) *bngpb.BeeName {
	return &bngpb.BeeName{
		Name: name,
	}
}

// NewBeeNameSuggestions - Create a new BeeNameSuggestions
func NewBeeNameSuggestions(suggestions []string) *bngpb.BeeNameSuggestions {
	return &bngpb.BeeNameSuggestions{
		Suggestions: suggestions,
	}
}
