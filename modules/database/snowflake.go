package database

import "github.com/kkrypt0nn/spaceflake"

var settings = initSnowflake()

// initSnowflake initializes the snowflake generator
func initSnowflake() spaceflake.GeneratorSettings {
	settings := spaceflake.NewGeneratorSettings()
	settings.BaseEpoch = 1706639400000 // January 30, 2024 12:30:00 PM Central/Regina
	settings.NodeID = 1
	settings.WorkerID = 1
	settings.Sequence = 0
	return settings
}

// GetSnowflake returns a new snowflake
func GetSnowflake() (*spaceflake.Spaceflake, error) {
	sf, err := spaceflake.Generate(settings)
	if err != nil {
		return nil, err
	}
	return sf, nil
}
