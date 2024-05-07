package archive

// Service interface for the archive service
type Service interface{}

// service implementation of the archive service
type service struct {
	store S3Store
}

// NewService creates a new archive service
func NewService(store S3Store) Service {
	return &service{store}
}

// PluginMod interface for a plugin/mod that can be converted to an MCMod
type PluginMod interface {
	ToMCMod(s Service) *MCMod
}
