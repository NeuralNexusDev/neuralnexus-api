package archive

// ArchiveItem is a struct that represents an item in the archive
type ArchiveItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Size     int64  `json:"size"`
	Links    []Link `json:"links"`
}

// Link is a struct that represents external links
type Link struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

// MinecraftMod is a struct that represents a Minecraft mod
type MinecraftMod struct {
	ArchiveItem
	ModID string `json:"mod_id"`
}
