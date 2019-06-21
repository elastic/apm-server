package reviewdog

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/google/go-github/github"
)

var _ CommentService = &GitHubPullRequest{}
var _ DiffService = &GitHubPullRequest{}

// GitHubPullRequest is a comment and diff service for GitHub PullRequest.
//
// API:
//	https://developer.github.com/v3/pulls/comments/#create-a-comment
//	POST /repos/:owner/:repo/pulls/:number/comments
type GitHubPullRequest struct {
	cli   *github.Client
	owner string
	repo  string
	pr    int
	sha   string

	muComments   sync.Mutex
	postComments []*Comment

	postedcs postedcomments

	// wd is working directory relative to root of repository.
	wd string
}

// NewGitHubPullReqest returns a new GitHubPullRequest service.
// GitHubPullRequest service needs git command in $PATH.
func NewGitHubPullReqest(cli *github.Client, owner, repo string, pr int, sha string) (*GitHubPullRequest, error) {
	workDir, err := gitRelWorkdir()
	if err != nil {
		return nil, fmt.Errorf("GitHubPullRequest needs 'git' command: %v", err)
	}
	return &GitHubPullRequest{
		cli:   cli,
		owner: owner,
		repo:  repo,
		pr:    pr,
		sha:   sha,
		wd:    workDir,
	}, nil
}

// Post accepts a comment and holds it. Flush method actually posts comments to
// GitHub in parallel.
func (g *GitHubPullRequest) Post(_ context.Context, c *Comment) error {
	c.Path = filepath.Join(g.wd, c.Path)
	g.muComments.Lock()
	defer g.muComments.Unlock()
	g.postComments = append(g.postComments, c)
	return nil
}

// Flush posts comments which has not been posted yet.
func (g *GitHubPullRequest) Flush(ctx context.Context) error {
	g.muComments.Lock()
	defer g.muComments.Unlock()

	if err := g.setPostedComment(ctx); err != nil {
		return err
	}
	return g.postAsReviewComment(ctx)
}

func (g *GitHubPullRequest) postAsReviewComment(ctx context.Context) error {
	comments := make([]*github.DraftReviewComment, 0, len(g.postComments))
	for _, c := range g.postComments {
		if g.postedcs.IsPosted(c, c.LnumDiff) {
			continue
		}
		cbody := commentBody(c)
		comments = append(comments, &github.DraftReviewComment{
			Path:     &c.Path,
			Position: &c.LnumDiff,
			Body:     &cbody,
		})
	}

	if len(comments) == 0 {
		return nil
	}

	// TODO(haya14busa): it might be useful to report overview results by "body"
	// field.
	review := &github.PullRequestReviewRequest{
		CommitID: &g.sha,
		Event:    github.String("COMMENT"),
		Comments: comments,
	}
	_, _, err := g.cli.PullRequests.CreateReview(ctx, g.owner, g.repo, g.pr, review)
	return err
}

func (g *GitHubPullRequest) setPostedComment(ctx context.Context) error {
	g.postedcs = make(postedcomments)
	cs, err := g.comment(ctx)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.Position == nil || c.Path == nil || c.Body == nil {
			// skip resolved comments. Or comments which do not have "path" nor
			// "body".
			continue
		}
		g.postedcs.AddPostedComment(c.GetPath(), c.GetPosition(), c.GetBody())
	}
	return nil
}

// Diff returns a diff of PullRequest.
func (g *GitHubPullRequest) Diff(ctx context.Context) ([]byte, error) {
	opt := github.RawOptions{Type: github.Diff}
	d, _, err := g.cli.PullRequests.GetRaw(ctx, g.owner, g.repo, g.pr, opt)
	if err != nil {
		return nil, err
	}
	return []byte(d), nil
}

// Strip returns 1 as a strip of git diff.
func (g *GitHubPullRequest) Strip() int {
	return 1
}

func (g *GitHubPullRequest) comment(ctx context.Context) ([]*github.PullRequestComment, error) {
	// https://developer.github.com/v3/guides/traversing-with-pagination/
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	comments, err := listAllPullRequestsComments(ctx, g.cli, g.owner, g.repo, g.pr, opts)
	if err != nil {
		return nil, err
	}
	return comments, nil
}

func listAllPullRequestsComments(ctx context.Context, cli *github.Client,
	owner, repo string, pr int, opts *github.PullRequestListCommentsOptions) ([]*github.PullRequestComment, error) {
	comments, resp, err := cli.PullRequests.ListComments(ctx, owner, repo, pr, opts)
	if err != nil {
		return nil, err
	}
	if resp.NextPage == 0 {
		return comments, nil
	}
	newOpts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{
			Page:    resp.NextPage,
			PerPage: opts.PerPage,
		},
	}
	restComments, err := listAllPullRequestsComments(ctx, cli, owner, repo, pr, newOpts)
	if err != nil {
		return nil, err
	}
	return append(comments, restComments...), nil
}
