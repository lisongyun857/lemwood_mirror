package browser

import (
    "errors"
    "fmt"
    "net/url"
    "regexp"
    "strings"

    "github.com/gocolly/colly/v2"
)

// ResolveRepoURL visits the given source URL and attempts to find a GitHub repo link.
// If source is already a github.com URL, it returns it directly.
// repoSelector: "regex:<pattern>" will match anchor hrefs with the regex.
// Otherwise treated as a CSS selector and first matching element's href is used.
func ResolveRepoURL(source string, repoSelector string) (string, error) {
    if source == "" {
        return "", errors.New("source url empty")
    }
    u, err := url.Parse(source)
    if err != nil {
        return "", fmt.Errorf("invalid source url: %w", err)
    }

    // If the source looks like a direct GitHub repo URL (https://github.com/owner/repo), return as-is
    if strings.Contains(u.Host, "github.com") {
        parts := strings.Split(strings.Trim(u.Path, "/"), "/")
        if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
            // Direct repo URL
            return "https://github.com/" + parts[0] + "/" + parts[1], nil
        }
        // Otherwise, it's a GitHub page (e.g., search/results). We will crawl anchors below.
    }

    c := colly.NewCollector(
        colly.MaxDepth(1),
        colly.AllowedDomains(u.Host),
    )
    var found string
    var re *regexp.Regexp
    cssSelector := "a"
    if repoSelector != "" {
        if strings.HasPrefix(repoSelector, "regex:") {
            pattern := strings.TrimPrefix(repoSelector, "regex:")
            compiled, err := regexp.Compile(pattern)
            if err != nil {
                return "", fmt.Errorf("invalid regex in repo_selector: %w", err)
            }
            re = compiled
            cssSelector = "a"
        } else {
            cssSelector = repoSelector
        }
    }

    // Default strict match: https://github.com/<owner>/<repo> or /<owner>/<repo>
    defaultAbsRe := regexp.MustCompile(`^https://github\.com/[^/]+/[^/#?]+$`)
    defaultRelRe := regexp.MustCompile(`^/[^/]+/[^/#?]+$`)

    c.OnHTML(cssSelector, func(e *colly.HTMLElement) {
        if found != "" {
            return
        }
        href := strings.TrimSpace(e.Attr("href"))
        if href == "" {
            return
        }
        // If custom regex provided
        if re != nil {
            if re.MatchString(href) {
                if strings.HasPrefix(href, "/") {
                    found = "https://github.com" + href
                } else {
                    found = href
                }
            }
            return
        }
        // Otherwise apply default matching against GitHub repo URL
        if defaultAbsRe.MatchString(href) {
            found = href
            return
        }
        if defaultRelRe.MatchString(href) {
            found = "https://github.com" + href
            return
        }
    })
    if err := c.Visit(source); err != nil {
        return "", fmt.Errorf("visit source: %w", err)
    }
    if found == "" {
        return "", errors.New("github repo url not found from source page")
    }
    return found, nil
}
