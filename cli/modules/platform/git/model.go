package git

import (
	"time"
)

// Status represents the git status of a repository
type Status struct {
	Branch        string   `json:"branch"`
	Remote        string   `json:"remote,omitempty"`
	Ahead         int      `json:"ahead"`
	Behind        int      `json:"behind"`
	IsClean       bool     `json:"is_clean"`
	HasUntracked  bool     `json:"has_untracked"`
	HasModified   bool     `json:"has_modified"`
	HasStaged     bool     `json:"has_staged"`
	Untracked     []string `json:"untracked,omitempty"`
	Modified      []string `json:"modified,omitempty"`
	Staged        []string `json:"staged,omitempty"`
	Deleted       []string `json:"deleted,omitempty"`
	Conflicts     []string `json:"conflicts,omitempty"`
}

// IsEmpty returns true if the status has no changes
func (s *Status) IsEmpty() bool {
	return s.IsClean && !s.HasUntracked && s.Ahead == 0 && s.Behind == 0
}

// ChangeCount returns the total number of changes
func (s *Status) ChangeCount() int {
	return len(s.Untracked) + len(s.Modified) + len(s.Staged) + len(s.Deleted)
}

// Commit represents a git commit
type Commit struct {
	Hash        string    `json:"hash"`
	ShortHash   string    `json:"short_hash"`
	Author      string    `json:"author"`
	AuthorEmail string    `json:"author_email"`
	Date        time.Time `json:"date"`
	Message     string    `json:"message"`
	Subject     string    `json:"subject"` // First line of message
}

// FileDiff represents a diff for a single file
type FileDiff struct {
	Path     string `json:"path"`
	OldPath  string `json:"old_path,omitempty"` // For renames
	Status   string `json:"status"`             // A, M, D, R, C
	IsBinary bool   `json:"is_binary"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch,omitempty"`
}

// Diff represents a git diff
type Diff struct {
	Files       []FileDiff `json:"files"`
	TotalAdded  int        `json:"total_added"`
	TotalRemoved int       `json:"total_removed"`
	FileCount   int        `json:"file_count"`
}

// Branch represents a git branch
type Branch struct {
	Name      string `json:"name"`
	IsRemote  bool   `json:"is_remote"`
	IsCurrent bool   `json:"is_current"`
	Commit    string `json:"commit"` // Short hash of HEAD
}

// Remote represents a git remote
type Remote struct {
	Name     string   `json:"name"`
	FetchURL string   `json:"fetch_url"`
	PushURL  string   `json:"push_url"`
}

// LogOptions represents options for git log
type LogOptions struct {
	MaxCount int    `json:"max_count"`
	Since    string `json:"since,omitempty"`
	Until    string `json:"until,omitempty"`
	Author   string `json:"author,omitempty"`
	Path     string `json:"path,omitempty"`
}

// DefaultLogOptions returns default log options
func DefaultLogOptions() LogOptions {
	return LogOptions{
		MaxCount: 20,
	}
}

// DiffOptions represents options for git diff
type DiffOptions struct {
	Staged   bool   `json:"staged"`
	Path     string `json:"path,omitempty"`
	Context  int    `json:"context"` // Number of context lines
}

// DefaultDiffOptions returns default diff options
func DefaultDiffOptions() DiffOptions {
	return DiffOptions{
		Context: 3,
	}
}
