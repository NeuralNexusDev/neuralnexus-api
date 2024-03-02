package projects

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// Globals
var githubToken string = os.Getenv("GITHUB_TOKEN")

var forgeModVersions = []string{
	"1.7.10",
	"1.8.9",
	"1.9.4",
	"1.10.2",
	"1.11.2",
	"1.12.2",
	"1.13.2",
	"1.14.4",
	"1.15.2",
	"1.16.5",
	"1.17.1",
	"1.18",
	"1.18.1",
	"1.18.2",
	"1.19",
	"1.19.1",
	"1.19.2",
	"1.19.3",
	"1.19.4",
	"1.20",
	"1.20.1",
	"1.20.2",
	"1.20.3",
	"1.20.4",
}

// Structs
type Release struct {
	TagName string `json:"tag_name"`
	URL     string `json:"html_url"`
}

// Forge Mod Updates Format
type ForgeModUpdates struct {
	Homepage string            `json:"homepage"`
	Promos   map[string]string `json:"promos"`
	V1_7_10  map[string]string `json:"1.7.10"`
	V1_8_9   map[string]string `json:"1.8.9"`
	V1_9_4   map[string]string `json:"1.9.4"`
	V1_10_2  map[string]string `json:"1.10.2"`
	V1_11_2  map[string]string `json:"1.11.2"`
	V1_12_2  map[string]string `json:"1.12.2"`
	V1_13_2  map[string]string `json:"1.13.2"`
	V1_14_4  map[string]string `json:"1.14.4"`
	V1_15_2  map[string]string `json:"1.15.2"`
	V1_16_5  map[string]string `json:"1.16.5"`
	V1_17_1  map[string]string `json:"1.17.1"`
	V1_18    map[string]string `json:"1.18"`
	V1_18_1  map[string]string `json:"1.18.1"`
	V1_18_2  map[string]string `json:"1.18.2"`
	V1_19    map[string]string `json:"1.19"`
	V1_19_1  map[string]string `json:"1.19.1"`
	V1_19_2  map[string]string `json:"1.19.2"`
	V1_19_3  map[string]string `json:"1.19.3"`
	V1_19_4  map[string]string `json:"1.19.4"`
	V1_20    map[string]string `json:"1.20"`
	V1_20_1  map[string]string `json:"1.20.1"`
	V1_20_2  map[string]string `json:"1.20.2"`
	V1_20_3  map[string]string `json:"1.20.3"`
	V1_20_4  map[string]string `json:"1.20.4"`
}

// Functions
func GetReleases(group string, project string) ([]Release, error) {
	githubURL := "https://api.github.com/repos/" + group + "/" + project + "/releases"
	req, err := http.NewRequest("GET", githubURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "neuralnexus-api")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var releases []Release
	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		return nil, err
	}

	return releases, nil
}

// Convert to Forge Mod Updates Format
func ConvertToForgeModUpdates(gitHubReleasesURL string, releases []Release) map[string]interface{} {
	forgeModUpdates := make(map[string]interface{})

	releaseMap := make(map[string]string)
	for _, release := range releases {
		releaseMap[release.TagName] = release.URL
	}

	promosMap := make(map[string]string)
	for _, version := range forgeModVersions {
		promosMap[version+"-latest"] = releases[0].URL
		promosMap[version+"-recommended"] = releases[0].URL
	}

	forgeModUpdates["homepage"] = gitHubReleasesURL
	forgeModUpdates["promos"] = promosMap
	for _, version := range forgeModVersions {
		forgeModUpdates[version] = releaseMap
	}

	return forgeModUpdates
}

// Handlers
func GetReleasesHandler(c echo.Context) error {
	group := c.Param("group")
	project := c.Param("project")

	format := c.QueryParam("format")

	releases, err := GetReleases(group, project)
	if err != nil {
		log.Println(err.Error())
		return c.JSON(http.StatusInternalServerError, err.Error())
	}

	if format == "forge-mod-updates" {
		gitHubReleasesURL := "https://github.com/" + group + "/" + project + "/releases"
		forgeModUpdates := ConvertToForgeModUpdates(gitHubReleasesURL, releases)
		return c.JSON(http.StatusOK, forgeModUpdates)
	}
	return c.JSON(http.StatusOK, releases)
}
