package gh

import (
    "context"
    "errors"
    "net/http"
    "strings"
    "time"

    github "github.com/google/go-github/v50/github"
    "golang.org/x/oauth2"
)

type Client struct {
	cli *github.Client
}

func NewClient(token string) *Client {
	var httpClient *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		httpClient = oauth2.NewClient(context.Background(), ts)
	}
	return &Client{cli: github.NewClient(httpClient)}
}

// ParseOwnerRepo extracts owner and repo from a full GitHub repo URL.
func ParseOwnerRepo(repoURL string) (string, string, error) {
	// Expected: https://github.com/<owner>/<repo>
	parts := strings.Split(strings.TrimPrefix(repoURL, "https://github.com/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("invalid repo url, need https://github.com/<owner>/<repo>")
	}
	owner := parts[0]
	repo := parts[1]
	return owner, repo, nil
}

// LatestRelease fetches only the latest release metadata.
func (c *Client) LatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error) {
    return c.cli.Repositories.GetLatestRelease(ctx, owner, repo)
}

// BackoffIfRateLimited checks response for rate limiting and sleeps if needed.
func BackoffIfRateLimited(resp *github.Response) {
    if resp == nil || resp.Rate.Remaining > 0 {
        return
    }
	reset := resp.Rate.Reset.Time
	// sleep until reset + small margin
	d := time.Until(reset) + 2*time.Second
	if d > 0 && d < 15*time.Minute {
		time.Sleep(d)
	}
}
