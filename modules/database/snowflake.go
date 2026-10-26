package database

import (
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/kkrypt0nn/spaceflake"
)

var settings spaceflake.GeneratorSettings

// InitSnowflake initializes the snowflake generator
func InitSnowflake(cfg config.Snowflake) {
	s := spaceflake.NewGeneratorSettings()
	s.BaseEpoch = 1706639400000 // January 30, 2024 12:30:00 PM Central/Regina
	s.NodeID = cfg.NodeId
	s.WorkerID = cfg.WorkerId
	s.Sequence = 0
	settings = s
}

// GenSnowflake returns a new snowflake
func GenSnowflake() (string, error) {
	sf, err := spaceflake.Generate(settings)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(sf.ID(), 10), nil
}
