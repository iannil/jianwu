package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"path/filepath"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/engine/grill"
)

// checkSlugConflict returns nil if the slug is available; an error if a book exists
// and force=false. If force=true, removes existing book dir before returning nil.
// Per Q21.A3.
func checkSlugConflict(wsRoot, slug string, force bool) error {
	bookDir := filepath.Join(wsRoot, "books", slug)
	info, err := os.Stat(bookDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", bookDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", bookDir)
	}
	if !force {
		return fmt.Errorf("book %q already exists at %s; use --force to overwrite", slug, bookDir)
	}
	if err := os.RemoveAll(bookDir); err != nil {
		return fmt.Errorf("remove existing book dir: %w", err)
	}
	return nil
}

// offerResume checks for incomplete sessions and asks the user whether to resume.
// Returns the session to resume (or nil to start fresh).
// Per Q11.A2.
func offerResume(repo *grill.Repository, prompt *TerminalPrompt) (*grill.Session, error) {
	incomplete, err := repo.ListIncomplete()
	if err != nil {
		return nil, fmt.Errorf("list incomplete sessions: %w", err)
	}
	if len(incomplete) == 0 {
		return nil, nil
	}
	fmt.Fprintf(prompt.Out, "\n检测到 %d 个未完成的 grill 会话:\n", len(incomplete))
	for i, s := range incomplete {
		firstTopic := s.Answers["topic"]
		if firstTopic == "" {
			firstTopic = "(未开始)"
		}
		fmt.Fprintf(prompt.Out, "  [%d] %s — %s\n", i+1, s.ID, firstTopic)
	}
	fmt.Fprintf(prompt.Out, "[回车=新会话 / 输入序号=恢复] ")
	reader := bufio.NewReader(prompt.In)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read resume choice: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(incomplete) {
		return nil, fmt.Errorf("invalid selection: %q", line)
	}
	return incomplete[idx-1], nil
}

// deriveSlugFromTopic produces a slug from the topic answer.
// Uses book.Slugify.
func deriveSlugFromTopic(topic string) string {
	return book.Slugify(topic)
}
