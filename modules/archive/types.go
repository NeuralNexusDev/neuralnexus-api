package archive

// ArchiveItem is a struct that represents an item in the archive
type ArchiveItem struct {
	ID       string   `db:"id" json:"id" xml:"id"`
	FileName string   `db:"file_name" json:"file_name" xml:"file_name"`
	Size     int64    `db:"size" json:"size" xml:"size"`
	MD5      string   `db:"md5" json:"md5" xml:"md5"`
	SHA1     string   `db:"sha1" json:"sha1" xml:"sha1"`
	SHA256   string   `db:"sha256" json:"sha256" xml:"sha256"`
	SHA512   string   `db:"sha512" json:"sha512" xml:"sha512"`
	Related  []string `db:"related" json:"related" xml:"related"`
	Links    []Link   `db:"links" json:"links" xml:"links"`
}

// Link is a struct that represents external links
type Link struct {
	Rel  string `db:"rel" json:"rel" xml:"rel"`
	Href string `db:"href" json:"href" xml:"href"`
}

// MCMod is a struct that represents a Minecraft mod
type MCMod struct {
	ArchiveItem
	ModID        string            `db:"mod_id" json:"mod_id" xml:"mod_id"`
	Name         string            `db:"name" json:"name" xml:"name"`
	Authors      []string          `db:"authors" json:"authors" xml:"authors"`
	Contributors []string          `db:"contributors" json:"contributors" xml:"contributors"`
	Platforms    []ModPlatform     `db:"platforms" json:"platforms" xml:"platforms"`
	Dependencies []MCModDependency `db:"dependencies" json:"dependencies" xml:"dependencies"`
}

// ModPlatform is a type that represents a platform for a Minecraft mod
type ModPlatform string

const (
	// ModPlatformBukkit is the Bukkit platform
	ModPlatformBukkit ModPlatform = "bukkit"
	// ModPlatformBungeeCord is the BungeeCord platform
	ModPlatformBungeeCord ModPlatform = "bungeecord"
	// ModPlatformForge is the Forge platform
	ModPlatformForge ModPlatform = "forge"
	// ModPlatformFabric is the Fabric platform
	ModPlatformFabric ModPlatform = "fabric"
	// ModPlatformSponge is the Sponge platform
	ModPlatformSponge ModPlatform = "sponge"
	// ModPlatformVelocity is the Velocity platform
	ModPlatformVelocity ModPlatform = "velocity"
)

// MCModDependency is a struct that represents a dependency of a Minecraft mod
type MCModDependency struct {
	ID       string `db:"id" json:"id" xml:"id"`
	Required bool   `db:"required" json:"required" xml:"required"`
	Version  string `db:"version" json:"version" xml:"version"`
}
