package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-github/v76/github"
)

type realClient struct {
	owner, repo string
	gh          *github.Client
	http        *http.Client
}

// New resolves a token from $GITHUB_TOKEN, falling back to `gh auth token`. One
// is effectively required: GraphQL rejects unauthenticated requests outright,
// and REST allows only 60/hour.
func New(repo string, logf func(format string, args ...any)) (Client, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}
	token, err := resolveToken()
	if err != nil {
		return nil, err
	}
	if token == "" && logf != nil {
		logf("no GitHub token found ($GITHUB_TOKEN or `gh auth token`); GraphQL calls will fail with 401 and REST is limited to 60 requests/hour")
	}

	// Not WithAuthToken: it installs its own transport and would drop the retry
	// wrapper, so retryTransport attaches the token instead.
	httpClient := retryingClient(token, logf)
	return &realClient{
		owner: owner,
		repo:  name,
		gh:    github.NewClient(httpClient),
		http:  httpClient,
	}, nil
}

func retryingClient(token string, logf func(format string, args ...any)) *http.Client {
	// No Client.Timeout: retryTransport times out per attempt instead.
	return &http.Client{
		Transport: &retryTransport{
			base:  http.DefaultTransport,
			token: token,
			sleep: time.Sleep,
			logf:  logf,
		},
	}
}

func splitRepo(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo %q, expected owner/name", s)
	}
	return parts[0], parts[1], nil
}

func resolveToken() (string, error) {
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t, nil
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *realClient) IssueURL(number int) string {
	return fmt.Sprintf("https://github.com/%s/%s/issues/%d", c.owner, c.repo, number)
}

// isCommitNotFound matches GitHub's 422 "No commit found for SHA", i.e. an
// unpushed commit. Matching on status alone survives message rewording.
func isCommitNotFound(err error) bool {
	var gerr *github.ErrorResponse
	if errors.As(err, &gerr) && gerr.Response != nil {
		return gerr.Response.StatusCode == http.StatusUnprocessableEntity
	}
	return false
}

func (c *realClient) Commit(ctx context.Context, sha string) (*Commit, error) {
	rc, _, err := c.gh.Repositories.GetCommit(ctx, c.owner, c.repo, sha, nil)
	if err != nil {
		if isCommitNotFound(err) {
			return nil, ErrCommitNotFound
		}
		return nil, fmt.Errorf("get commit %s: %w", sha, err)
	}
	out := &Commit{SHA: sha}
	if a := rc.GetAuthor(); a != nil {
		out.AuthorLogin = a.GetLogin()
	}
	for _, f := range rc.Files {
		if p := f.GetFilename(); p != "" {
			out.Files = append(out.Files, p)
		}
	}
	return out, nil
}

func (c *realClient) PullsForCommit(ctx context.Context, sha string) ([]*PullRequest, error) {
	prs, _, err := c.gh.PullRequests.ListPullRequestsWithCommit(ctx, c.owner, c.repo, sha, nil)
	if err != nil {
		if isCommitNotFound(err) {
			return nil, ErrCommitNotFound
		}
		return nil, fmt.Errorf("list PRs for commit %s: %w", sha, err)
	}
	out := make([]*PullRequest, 0, len(prs))
	for _, pr := range prs {
		labels := make([]string, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			if name := l.GetName(); name != "" {
				labels = append(labels, name)
			}
		}
		out = append(out, &PullRequest{
			Number: pr.GetNumber(),
			URL:    pr.GetHTMLURL(),
			Labels: labels,
		})
	}
	return out, nil
}

// closingIssuesQuery is the only GraphQL query here; REST does not expose the
// PR "Development" panel.
const closingIssuesQuery = `query($owner:String!,$name:String!,$number:Int!){
  repository(owner:$owner,name:$name){
    pullRequest(number:$number){
      closingIssuesReferences(first:20){
        nodes{
          number
          url
          author{login}
        }
      }
    }
  }
}`

func (c *realClient) ClosingIssues(ctx context.Context, prNumber int) ([]*Issue, error) {
	body, err := json.Marshal(map[string]any{
		"query": closingIssuesQuery,
		"variables": map[string]any{
			"owner":  c.owner,
			"name":   c.repo,
			"number": prNumber,
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graphql closingIssuesReferences PR %d: %w", prNumber, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("graphql closingIssuesReferences PR %d: status 401: GitHub's GraphQL API requires authentication — set $GITHUB_TOKEN or run `gh auth login` (the public_repo scope is enough): %s",
				prNumber, strings.TrimSpace(string(b)))
		}
		return nil, fmt.Errorf("graphql closingIssuesReferences PR %d: status %d: %s", prNumber, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var payload struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ClosingIssuesReferences struct {
						Nodes []struct {
							Number int    `json:"number"`
							URL    string `json:"url"`
							Author struct {
								Login string `json:"login"`
							} `json:"author"`
						} `json:"nodes"`
					} `json:"closingIssuesReferences"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode graphql response: %w", err)
	}
	if len(payload.Errors) > 0 {
		msgs := make([]string, len(payload.Errors))
		for i, e := range payload.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("graphql errors: %s", strings.Join(msgs, "; "))
	}
	nodes := payload.Data.Repository.PullRequest.ClosingIssuesReferences.Nodes
	out := make([]*Issue, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &Issue{Number: n.Number, URL: n.URL, ReporterLogin: n.Author.Login})
	}
	return out, nil
}
