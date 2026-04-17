package zk

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type Note struct {
	Filename     string         `json:"filename"`
	FilenameStem string         `json:"filenameStem"`
	Path         string         `json:"path"`
	AbsPath      string         `json:"absPath"`
	Title        string         `json:"title"`
	Lead         string         `json:"lead"`
	Body         string         `json:"body"`
	Snippets     []string       `json:"snippets"`
	RawContent   string         `json:"rawContent"`
	WordCount    int            `json:"wordCount"`
	Tags         []string       `json:"tags"`
	Metadata     map[string]any `json:"metadata"`
	Created      time.Time      `json:"created"`
	Modified     time.Time      `json:"modified"`
	Checksum     string         `json:"checksum"`
}

type Tag struct {
	ID        int    `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	NoteCount int    `json:"noteCount"`
}

type Client struct {
	notebookPath string
}

func NewClient(notebookPath string) *Client {
	return &Client{notebookPath: notebookPath}
}

func (c *Client) List(query string, tags []string) ([]Note, error) {
	args := []string{"list", "--format", "json", "--notebook", c.notebookPath}
	if query != "" {
		args = append(args, "--match", query)
	}
	for _, t := range tags {
		args = append(args, "--tag", t)
	}
	out, err := exec.Command("zk", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("zk list: %w", err)
	}
	var notes []Note
	if err := json.Unmarshal(out, &notes); err != nil {
		return nil, fmt.Errorf("zk list parse: %w", err)
	}
	return notes, nil
}

func (c *Client) TagList() ([]Tag, error) {
	args := []string{"tag", "list", "--format", "json", "--notebook", c.notebookPath}
	out, err := exec.Command("zk", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("zk tag list: %w", err)
	}
	var tags []Tag
	if err := json.Unmarshal(out, &tags); err != nil {
		return nil, fmt.Errorf("zk tag list parse: %w", err)
	}
	return tags, nil
}
