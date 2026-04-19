package model

// FileNode is one entry in the sidebar folder tree.
type FileNode struct {
	Name     string
	Path     string // non-empty for notes
	IsDir    bool
	IsActive bool
	IsOpen   bool // dir should render <details open>
	Children []*FileNode
}

// BreadcrumbSegment is one folder step in a note's path.
type BreadcrumbSegment struct {
	Name       string
	FolderPath string // e.g. "notes" or "notes/ai"
}

// FolderEntry is one item (file or subdirectory) in a folder listing.
type FolderEntry struct {
	Name  string
	Path  string
	Title string
	IsDir bool
}
