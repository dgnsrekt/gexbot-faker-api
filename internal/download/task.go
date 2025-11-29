package download

import (
	"fmt"
	"path/filepath"
)

type Task struct {
	Ticker   string
	Package  string
	Category string
	Date     string
}

func (t Task) APIPath() string {
	return fmt.Sprintf("%s/%s/%s/%s", t.Ticker, t.Package, t.Category, t.Date)
}

func (t Task) OutputPath(baseDir string) string {
	return filepath.Join(baseDir, t.Date, t.Ticker, t.Package, t.Category+".json")
}

func (t Task) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", t.Date, t.Ticker, t.Package, t.Category)
}

type TaskResult struct {
	Task      Task
	Success   bool
	Skipped   bool
	NotFound  bool
	BytesSize int64
	Error     error
}
