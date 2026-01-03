package git

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Repository wraps go-git repository with caching
type Repository struct {
	path string
	repo *git.Repository

	// Cache for worktree status (expensive operation)
	statusCache     git.Status
	statusCacheTime time.Time
	statusCacheTTL  time.Duration
	statusMu        sync.RWMutex
}

// OpenRepository opens a git repository at the given path
func OpenRepository(path string) (*Repository, error) {
	// Find the .git directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	repo, err := git.PlainOpen(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	return &Repository{
		path:           absPath,
		repo:           repo,
		statusCacheTTL: 1 * time.Second, // Cache worktree status for 1 second
	}, nil
}

// getWorktreeStatus returns cached worktree status or fetches fresh
func (r *Repository) getWorktreeStatus() (git.Status, error) {
	r.statusMu.RLock()
	if r.statusCache != nil && time.Since(r.statusCacheTime) < r.statusCacheTTL {
		status := r.statusCache
		r.statusMu.RUnlock()
		return status, nil
	}
	r.statusMu.RUnlock()

	// Cache miss - fetch fresh status
	worktree, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Update cache
	r.statusMu.Lock()
	r.statusCache = status
	r.statusCacheTime = time.Now()
	r.statusMu.Unlock()

	return status, nil
}

// InvalidateCache clears the status cache
func (r *Repository) InvalidateCache() {
	r.statusMu.Lock()
	r.statusCache = nil
	r.statusMu.Unlock()
}

// IsRepository checks if a path is a git repository
func IsRepository(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

// GetStatus returns the current git status (uses cache)
func (r *Repository) GetStatus() (*Status, error) {
	status, err := r.getWorktreeStatus()
	if err != nil {
		return nil, err
	}

	result := &Status{
		IsClean:   status.IsClean(),
		Untracked: []string{},
		Modified:  []string{},
		Staged:    []string{},
		Deleted:   []string{},
	}

	// Get current branch
	head, err := r.repo.Head()
	if err == nil {
		result.Branch = head.Name().Short()
	}

	// Parse status
	for file, fileStatus := range status {
		switch fileStatus.Staging {
		case git.Added, git.Copied:
			result.Staged = append(result.Staged, file)
			result.HasStaged = true
		case git.Modified:
			result.Staged = append(result.Staged, file)
			result.HasStaged = true
		case git.Deleted:
			result.Staged = append(result.Staged, file)
			result.HasStaged = true
		}

		switch fileStatus.Worktree {
		case git.Untracked:
			result.Untracked = append(result.Untracked, file)
			result.HasUntracked = true
		case git.Modified:
			result.Modified = append(result.Modified, file)
			result.HasModified = true
		case git.Deleted:
			result.Deleted = append(result.Deleted, file)
			result.HasModified = true
		}
	}

	// Sort file lists for consistent ordering
	sort.Strings(result.Staged)
	sort.Strings(result.Modified)
	sort.Strings(result.Untracked)
	sort.Strings(result.Deleted)

	// Get remote info
	remotes, err := r.repo.Remotes()
	if err == nil && len(remotes) > 0 {
		result.Remote = remotes[0].Config().Name
	}

	// TODO: Calculate ahead/behind (requires fetching remote refs)

	return result, nil
}

// GetLog returns the commit log
func (r *Repository) GetLog(opts LogOptions) ([]Commit, error) {
	if opts.MaxCount <= 0 {
		opts.MaxCount = 20
	}

	logOpts := &git.LogOptions{
		Order: git.LogOrderCommitterTime,
	}

	if opts.Path != "" {
		logOpts.FileName = &opts.Path
	}

	commitIter, err := r.repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}

	var commits []Commit
	count := 0

	err = commitIter.ForEach(func(c *object.Commit) error {
		if count >= opts.MaxCount {
			return fmt.Errorf("stop") // Stop iteration
		}

		commit := Commit{
			Hash:        c.Hash.String(),
			ShortHash:   c.Hash.String()[:7],
			Author:      c.Author.Name,
			AuthorEmail: c.Author.Email,
			Date:        c.Author.When,
			Message:     c.Message,
			Subject:     strings.Split(c.Message, "\n")[0],
		}

		commits = append(commits, commit)
		count++
		return nil
	})

	if err != nil && err.Error() != "stop" {
		return nil, fmt.Errorf("failed to iterate log: %w", err)
	}

	return commits, nil
}

// GetDiff returns the current diff (uses cached status)
func (r *Repository) GetDiff(opts DiffOptions) (*Diff, error) {
	status, err := r.getWorktreeStatus()
	if err != nil {
		return nil, err
	}

	diff := &Diff{
		Files: []FileDiff{},
	}

	for file, fileStatus := range status {
		if opts.Path != "" && file != opts.Path {
			continue
		}

		var fd FileDiff
		fd.Path = file

		// Determine status
		if opts.Staged {
			switch fileStatus.Staging {
			case git.Added:
				fd.Status = "A"
			case git.Modified:
				fd.Status = "M"
			case git.Deleted:
				fd.Status = "D"
			case git.Renamed:
				fd.Status = "R"
			case git.Copied:
				fd.Status = "C"
			default:
				continue // Skip if not staged
			}
		} else {
			switch fileStatus.Worktree {
			case git.Modified:
				fd.Status = "M"
			case git.Deleted:
				fd.Status = "D"
			case git.Untracked:
				fd.Status = "?"
			default:
				continue // Skip if not modified in worktree
			}
		}

		diff.Files = append(diff.Files, fd)
		diff.FileCount++
	}

	return diff, nil
}

// GetBranches returns all branches
func (r *Repository) GetBranches() ([]Branch, error) {
	var branches []Branch

	// Get current branch
	head, _ := r.repo.Head()
	currentBranch := ""
	if head != nil {
		currentBranch = head.Name().Short()
	}

	// Get local branches
	branchIter, err := r.repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	err = branchIter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		branches = append(branches, Branch{
			Name:      name,
			IsRemote:  false,
			IsCurrent: name == currentBranch,
			Commit:    ref.Hash().String()[:7],
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to iterate branches: %w", err)
	}

	return branches, nil
}

// GetRemotes returns all remotes
func (r *Repository) GetRemotes() ([]Remote, error) {
	remotes, err := r.repo.Remotes()
	if err != nil {
		return nil, fmt.Errorf("failed to get remotes: %w", err)
	}

	var result []Remote
	for _, remote := range remotes {
		cfg := remote.Config()
		r := Remote{
			Name: cfg.Name,
		}
		if len(cfg.URLs) > 0 {
			r.FetchURL = cfg.URLs[0]
			r.PushURL = cfg.URLs[0]
		}
		result = append(result, r)
	}

	return result, nil
}

// GetHead returns the current HEAD commit
func (r *Repository) GetHead() (*Commit, error) {
	head, err := r.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	return &Commit{
		Hash:        commit.Hash.String(),
		ShortHash:   commit.Hash.String()[:7],
		Author:      commit.Author.Name,
		AuthorEmail: commit.Author.Email,
		Date:        commit.Author.When,
		Message:     commit.Message,
		Subject:     strings.Split(commit.Message, "\n")[0],
	}, nil
}

// Path returns the repository path
func (r *Repository) Path() string {
	return r.path
}
