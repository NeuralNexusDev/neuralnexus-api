package database

import (
	"strconv"

	"github.com/kkrypt0nn/spaceflake"
)

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

// GenSnowflake returns a new snowflake
func GenSnowflake() (string, error) {
	sf, err := spaceflake.Generate(settings)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(sf.ID(), 10), nil
}
