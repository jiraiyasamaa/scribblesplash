package dungeon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"scribblesplash/internal/models"
)

type Manager struct {
	dungeonDir  string
	articlesDir string
}

func New(dungeonDir, articlesDir string) *Manager {
	os.MkdirAll(dungeonDir, 0755)
	return &Manager{
		dungeonDir:  dungeonDir,
		articlesDir: articlesDir,
	}
}

type Entry struct {
	Title      string
	Slug       string
	Date       string
	Category   string
	Excerpt    string
	SourceYear string
}

func (m *Manager) Delete(slug string) error {
	entries, err := os.ReadDir(m.articlesDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		year := entry.Name()
		yearDir := filepath.Join(m.articlesDir, year)
		files, err := os.ReadDir(yearDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() && strings.TrimSuffix(f.Name(), ".md") == slug {
				src := filepath.Join(yearDir, f.Name())
				dstDir := filepath.Join(m.dungeonDir, year)
				os.MkdirAll(dstDir, 0755)
				dst := filepath.Join(dstDir, f.Name())
				if err := os.Rename(src, dst); err != nil {
					return fmt.Errorf("moving to dungeon: %w", err)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("article %q not found", slug)
}

func (m *Manager) List() ([]Entry, error) {
	var entries []Entry
	yearDirs, err := os.ReadDir(m.dungeonDir)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}
	for _, yearDir := range yearDirs {
		if !yearDir.IsDir() {
			continue
		}
		year := yearDir.Name()
		dir := filepath.Join(m.dungeonDir, year)
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, f.Name()))
			if err != nil {
				continue
			}
			var article models.Article
			parts := strings.SplitN(string(data), "---", 3)
			var frontmatter string
			if len(parts) >= 3 {
				frontmatter = parts[1]
			}
			if frontmatter != "" {
				yaml.Unmarshal([]byte(frontmatter), &article)
			}
			slug := strings.TrimSuffix(f.Name(), ".md")
			if article.Slug == "" {
				article.Slug = slug
			}
			entries = append(entries, Entry{
				Title:      article.Title,
				Slug:       article.Slug,
				Date:       article.Date,
				Category:   article.Category,
				Excerpt:    article.Excerpt,
				SourceYear: year,
			})
		}
	}
	return entries, nil
}

func (m *Manager) Count() int {
	entries, _ := m.List()
	return len(entries)
}

func (m *Manager) Restore(slug string) error {
	yearDirs, err := os.ReadDir(m.dungeonDir)
	if err != nil {
		return err
	}
	for _, yearDir := range yearDirs {
		if !yearDir.IsDir() {
			continue
		}
		year := yearDir.Name()
		dir := filepath.Join(m.dungeonDir, year)
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() && strings.TrimSuffix(f.Name(), ".md") == slug {
				src := filepath.Join(dir, f.Name())
				dstDir := filepath.Join(m.articlesDir, year)
				os.MkdirAll(dstDir, 0755)
				dst := filepath.Join(dstDir, f.Name())
				if err := os.Rename(src, dst); err != nil {
					return fmt.Errorf("restoring from dungeon: %w", err)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("article %q not found in dungeon", slug)
}

func (m *Manager) PermanentDelete(slug string) error {
	yearDirs, err := os.ReadDir(m.dungeonDir)
	if err != nil {
		return err
	}
	for _, yearDir := range yearDirs {
		if !yearDir.IsDir() {
			continue
		}
		year := yearDir.Name()
		dir := filepath.Join(m.dungeonDir, year)
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() && strings.TrimSuffix(f.Name(), ".md") == slug {
				return os.Remove(filepath.Join(dir, f.Name()))
			}
		}
	}
	return fmt.Errorf("article %q not found in dungeon", slug)
}
