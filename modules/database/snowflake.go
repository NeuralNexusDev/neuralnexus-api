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
	s.BaseEpoch = cfg.BaseEpoch
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
