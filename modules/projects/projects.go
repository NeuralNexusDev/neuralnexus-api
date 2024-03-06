package projects

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// -------------- Globals --------------
var (
	githubToken string = os.Getenv("GITHUB_TOKEN")

	forgeModVersions = []string{
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
)

// Structs
type Release struct {
	TagName string `json:"tag_name"`
	URL     string `json:"html_url"`
}

// Functions
func getReleases(group string, project string) ([]Release, error) {
	if githubToken == "" {
		return nil, errors.New("GITHUB_TOKEN is not set")
	}

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
func ConvertToFMLFormat(gitHubReleasesURL string, releases []Release) map[string]interface{} {
	fmlFormat := make(map[string]interface{})

	releaseMap := make(map[string]string)
	for _, release := range releases {
		versionTagName := strings.Split(release.TagName, "v")[1]
		releaseMap[versionTagName] = release.URL
	}

	promosMap := make(map[string]string)
	for _, version := range forgeModVersions {
		promosMap[version+"-latest"] = releases[0].URL
		promosMap[version+"-recommended"] = releases[0].URL
	}

	fmlFormat["homepage"] = gitHubReleasesURL
	fmlFormat["promos"] = promosMap
	for _, version := range forgeModVersions {
		fmlFormat[version] = releaseMap
	}

	return fmlFormat
}

// Handlers
func GetReleasesHandler(c echo.Context) error {
	group := c.Param("group")
	project := c.Param("project")

	format := c.QueryParam("format")

	releases, err := getReleases(group, project)
	if err != nil {
		log.Println(err.Error())
		return c.JSON(http.StatusInternalServerError, err.Error())
	}

	if format == "fml" {
		gitHubReleasesURL := "https://github.com/" + group + "/" + project + "/releases"
		forgeModUpdates := ConvertToFMLFormat(gitHubReleasesURL, releases)
		return c.JSON(http.StatusOK, forgeModUpdates)
	}
	return c.JSON(http.StatusOK, releases)
}
