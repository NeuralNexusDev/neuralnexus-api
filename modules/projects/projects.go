package projects

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/goccy/go-json"
)

// -------------- Globals --------------
var (
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
		"1.20.5",
		"1.20.6",
		"1.21.1",
		"1.21.2",
		"1.21.3",
		"1.21.4",
		"1.21.5",
	}
)

// -------------- Structs --------------

type Release struct {
	TagName string `json:"tag_name"`
	URL     string `json:"html_url"`
}

var ErrGitHubTokenUnset = errors.New("GITHUB_TOKEN is not set")

// ErrReleasesNotFound is returned when GitHub answers 404 for the repository.
var ErrReleasesNotFound = errors.New("releases not found")

// ErrGitHubRateLimited is returned when GitHub answers 403 or 429.
var ErrGitHubRateLimited = errors.New("github is not accepting requests")

// ErrGitHubStatus is returned when GitHub answers any other non-200 status.
var ErrGitHubStatus = errors.New("github returned an unexpected status")

const (
	msgGitHubTokenUnset    = "GITHUB_TOKEN is not set"
	msgFailedToGetReleases = "Failed to get releases"
	msgReleasesNotFound    = "Releases not found"
	msgGitHubRateLimited   = "GitHub is not accepting requests right now. Try again later"
	msgGitHubBadStatus     = "GitHub returned an unexpected response"
)

// -------------- Functions --------------
func getReleases(ctx context.Context, cfg config.GitHub, group, project string) ([]Release, error) {
	githubURL := cfg.APIEndpoint + "/repos/" + url.PathEscape(group) + "/" + url.PathEscape(project) + "/releases"
	req, err := http.NewRequestWithContext(ctx, "GET", githubURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "neuralnexus-api")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrReleasesNotFound
	case http.StatusForbidden, http.StatusTooManyRequests:
		reqctx.Log(ctx, "GitHub releases request refused:\n\t", resp.Status)
		return nil, ErrGitHubRateLimited
	default:
		reqctx.Log(ctx, "GitHub releases request refused:\n\t", resp.Status)
		return nil, ErrGitHubStatus
	}

	var releases []Release
	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		return nil, err
	}

	return releases, nil
}

// ConvertToFMLFormat Convert to Forge Mod Updates Format
func ConvertToFMLFormat(gitHubReleasesURL string, releases []Release) map[string]interface{} {
	fmlFormat := make(map[string]interface{})

	releaseMap := make(map[string]string)
	for _, release := range releases {
		versionTagName := strings.TrimPrefix(release.TagName, "v")
		releaseMap[versionTagName] = release.URL
	}

	promosMap := make(map[string]string)
	if len(releases) > 0 {
		for _, version := range forgeModVersions {
			promosMap[version+"-latest"] = releases[0].URL
			promosMap[version+"-recommended"] = releases[0].URL
		}
	}

	fmlFormat["homepage"] = gitHubReleasesURL
	fmlFormat["promos"] = promosMap
	for _, version := range forgeModVersions {
		fmlFormat[version] = releaseMap
	}

	return fmlFormat
}

// GetReleasesHandler - Get the releases
func GetReleasesHandler(cfg config.GitHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		group := r.PathValue("group")
		project := r.PathValue("project")

		format := r.URL.Query().Get("format")

		releases, err := getReleases(r.Context(), cfg, group, project)
		if err != nil {
			reqctx.Log(r.Context(), err.Error())
			switch err {
			case ErrGitHubTokenUnset:
				http.Error(w, msgGitHubTokenUnset, http.StatusInternalServerError)
			case ErrReleasesNotFound:
				http.Error(w, msgReleasesNotFound, http.StatusNotFound)
			case ErrGitHubRateLimited:
				http.Error(w, msgGitHubRateLimited, http.StatusServiceUnavailable)
			case ErrGitHubStatus:
				http.Error(w, msgGitHubBadStatus, http.StatusBadGateway)
			default:
				http.Error(w, msgFailedToGetReleases, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if format == "fml" {
			gitHubReleasesURL := "https://github.com/" + group + "/" + project + "/releases"
			forgeModUpdates := ConvertToFMLFormat(gitHubReleasesURL, releases)
			if err := json.NewEncoder(w).Encode(forgeModUpdates); err != nil {
				reqctx.Log(r.Context(), "Failed to write releases:\n\t", err.Error())
			}
			return
		}
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			reqctx.Log(r.Context(), "Failed to write releases:\n\t", err.Error())
		}
	}
}
