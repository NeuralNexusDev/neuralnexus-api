package archive

// BukkitPlugin is a struct that represents the plugin.yml file of a Bukkit plugin
type BukkitPlugin struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Author      string   `yaml:"author"`
	Authors     []string `yaml:"authors"`
	Description string   `yaml:"description"`
	Website     string   `yaml:"website"`
	Main        string   `yaml:"main"`
	Depend      []string `yaml:"depend"`
	Depends     []string `yaml:"depends"`
	SoftDepend  []string `yaml:"softdepend"`
	SoftDepends []string `yaml:"softdepends"`
	LoadBefore  []string `yaml:"loadbefore"`
	Load        string   `yaml:"load"`
}

// BungeeCordPlugin is a struct that represents the bungee.yml/plugin.yml file of a BungeeCord plugin
type BungeeCordPlugin struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Author      string   `yaml:"author"`
	Authors     []string `yaml:"authors"`
	Description string   `yaml:"description"`
	Website     string   `yaml:"website"`
	Main        string   `yaml:"main"`
	Depend      []string `yaml:"depend"`
	Depends     []string `yaml:"depends"`
	SoftDepend  []string `yaml:"softdepend"`
	SoftDepends []string `yaml:"softdepends"`
	LoadBefore  []string `yaml:"loadbefore"`
}

type (
	// FabricMod is a struct that represents the fabric.mod.json file of a Fabric mod
	FabricMod struct {
		SchemaVersion int `json:"schemaVersion"`

		// Mandatory fields
		ID      string `json:"id"`
		Version string `json:"version"`

		// Optional fields - Mod Loading
		Environment string            `json:"environment"` // client, server, *. can also be a list, will ignore for this implementation
		EntryPoints map[string]string `json:"entrypoints"`
		Jars        []struct {
			File string `json:"file"`
		} `json:"jars"`
		LanguageAdapters map[string]string `json:"languageAdapters"`
		Mixins           []string          `json:"mixins"`
		AccessWidener    string            `json:"accessWidener"`

		// Optional fields - Dependency Resolution
		Depends    map[string]string `json:"depends"`
		Recommends map[string]string `json:"recommends"`
		Suggests   map[string]string `json:"suggests"`
		Conflicts  map[string]string `json:"conflicts"`
		Breaks     map[string]string `json:"breaks"`

		// Optional fields - Metadata
		Name         string                   `json:"name"`
		Description  string                   `json:"description"`
		Authors      []FabricPerson           `json:"authors"`
		Contributors []FabricPerson           `json:"contributors"`
		Contact      FabricContactInformation `json:"contact"`
		License      string                   `json:"license"` // Can also be a list, will ignore for this implementation
		Icon         string                   `json:"icon"`    // Can also be a map, will ignore for this implementation
	}

	// FabricPerson - author or contributor
	FabricPerson struct {
		Name    string                   `json:"name"`
		Contact FabricContactInformation `json:"contact"`
	}

	// FabricContactInformation - contact information for a person
	FabricContactInformation struct {
		Email    string `json:"email"`
		IRC      string `json:"irc"`
		HomePage string `json:"homepage"`
		Issues   string `json:"issues"`
		Sources  string `json:"sources"`
	}
)

// ForgeLegacyMod is a struct that represents the mcmod.info file of a Forge mod
type ForgeLegacyMod []struct {
	ModID                    string   `json:"modid"`
	Name                     string   `json:"name"`
	Description              string   `json:"description"`
	Version                  string   `json:"version"`
	MCVersion                string   `json:"mcversion"`
	URL                      string   `json:"url"`
	UpdateURL                string   `json:"updateUrl"`
	UpdateJSON               string   `json:"updateJSON"`
	AuthorList               []string `json:"authorList"`
	Credits                  string   `json:"credits"`
	LogoFile                 string   `json:"logoFile"`
	Screenshots              []string `json:"screenshots"`
	Parent                   string   `json:"parent"`
	UseDependencyInformation bool     `json:"useDependencyInformation"`
	RequiredMods             []string `json:"requiredMods"`
	Dependencies             []string `json:"dependencies"`
	Dependants               []string `json:"dependants"`
}

// MCMeta is a struct that represents the pack.mcmeta file
type MCMeta struct {
	Pack struct {
		PackFormat  int    `json:"pack_format"`
		Description string `json:"description"`
	} `json:"pack"`
}

type (
	// ForgeMod is a struct that represents the META-INF/mods.toml file of a Forge mod
	ForgeMod struct {
		// Mandatory non-mod-specific properties
		ModLoader     string `toml:"modLoader"`
		LoaderVersion string `toml:"loaderVersion"`
		License       string `toml:"license"`

		// Optional non-mod-specific properties
		ShowAsResourcePack bool `toml:"showAsResourcePack"`
		Properties         map[string]interface{}
		IssueTrackerURL    string `toml:"issueTrackerURL"`

		Mods         []ForgeModInfo
		Dependencies map[string][]ForgeModDependency
	}

	// ModInfo represents the [[mods]] section of the mods.toml file
	ForgeModInfo struct {
		// Mandatory Mod Properties
		ModID string `toml:"modId"`

		// Optional Mod Properties
		Namespace     string                 `toml:"namespace"`
		Version       string                 `toml:"version"`
		DisplayName   string                 `toml:"displayName"`
		Description   string                 `toml:"description"`
		LogoFile      string                 `toml:"logoFile"`
		LogoBlur      bool                   `toml:"logoBlur"`
		UpdateJSONURL string                 `toml:"updateJSONURL"`
		ModProperties map[string]interface{} `toml:"modProperties"`
		Credits       string                 `toml:"credits"`
		Authors       string                 `toml:"authors"`
		DisplayURL    string                 `toml:"displayURL"`
		DisplayTest   string                 `toml:"displayTest"`
	}

	// ModDependency represents the [[dependencies.modId]] section of the mods.toml file
	ForgeModDependency struct {
		ModID        string `toml:"modId"`
		Mandatory    bool   `toml:"mandatory"`
		VersionRange string `toml:"versionRange"`
		Ordering     string `toml:"ordering"`
		Side         string `toml:"side"`
	}
)

type (
	// SpongePlugin is a struct that represents the META-INF/sponge_plugins.json file of a Sponge plugin
	SpongePlugin struct {
		// Required properties
		Loader  SpongeLoader `json:"loader"`
		License string       `json:"license"`

		// Optional properties (need at least one plugin)
		Global  SpongeGlobal       `json:"global"`
		Plugins []SpongePluginInfo `json:"plugins"`
	}

	// SpongeLoader is a struct that represents the loader property of a Sponge plugin
	SpongeLoader struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	// SpongeGlobal is a struct that represents the global property of a Sponge plugin
	SpongeGlobal struct {
		Version      string              `json:"version"`
		Links        SpongeLinks         `json:"links"`
		Contributors []SpongeContributor `json:"contributors"`
		Dependencies []SpongeDependency  `json:"dependencies"`
		Branding     SpongeBranding      `json:"branding"`
	}

	// SpongeLinks is a struct that represents the links property of a Sponge plugin
	SpongeLinks struct {
		Homepage string `json:"homepage"`
		Source   string `json:"source"`
		Issues   string `json:"issues"`
	}

	// SpongeContributor is a struct that represents a contributor to a Sponge plugin
	SpongeContributor struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	// SpongeDependency is a struct that represents a dependency of a Sponge plugin
	SpongeDependency struct {
		ID        string `json:"id"`
		Version   string `json:"version"`
		LoadOrder string `json:"load-order"`
		Optional  bool   `json:"optional"`
	}

	// SpongeBranding is a struct that represents the branding property of a Sponge plugin
	SpongeBranding struct {
		Logo string `json:"logo"`
		Icon string `json:"icon"`
	}

	// SpongePluginInfo is a struct that represents a plugin in a Sponge plugin
	// Note: Version and Contributors are not required if global.version and global.contributors are set
	SpongePluginInfo struct {
		SpongeGlobal
		ID          string `json:"id"`
		Name        string `json:"name"`
		Entrypoint  string `json:"entrypoint"`
		Description string `json:"description"`
	}
)
