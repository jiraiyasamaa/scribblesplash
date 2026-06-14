package storage

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"scribblesplash/internal/models"

	"gopkg.in/yaml.v3"
)

type Store struct {
	Articles []models.Article
	BySlug   map[string]*models.Article
}

func New(articlesDir string) (*Store, error) {
	s := &Store{
		BySlug: make(map[string]*models.Article),
	}

	err := filepath.Walk(articlesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		article, err := parseArticle(path)
		if err != nil {
			return fmt.Errorf("error parsing %s: %w", path, err)
		}

		s.Articles = append(s.Articles, *article)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking articles dir: %w", err)
	}

	sort.Slice(s.Articles, func(i, j int) bool {
		di, _ := time.Parse("2006-01-02", s.Articles[i].Date)
		dj, _ := time.Parse("2006-01-02", s.Articles[j].Date)
		return di.After(dj)
	})

	for i := range s.Articles {
		s.BySlug[s.Articles[i].Slug] = &s.Articles[i]
	}

	return s, nil
}

func parseArticle(path string) (*models.Article, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)

	parts := strings.SplitN(content, "---", 3)
	var frontmatter, body string
	if len(parts) >= 3 {
		frontmatter = parts[1]
		body = parts[2]
	} else {
		body = content
	}

	article := &models.Article{}
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), article); err != nil {
			return nil, fmt.Errorf("error parsing frontmatter: %w", err)
		}
	}

	body = strings.TrimSpace(body)
	article.Content = body
	article.BodyHTML = renderMarkdownToHTML(body)
	article.WordCount = countWords(body)

	if article.Date != "" {
		t, err := time.Parse("2006-01-02", article.Date)
		if err == nil {
			article.Year = t.Year()
			article.Month = t.Month()
		} else {
			t2, err2 := time.Parse("2006-01-02 15:04:05", article.Date)
			if err2 == nil {
				article.Year = t2.Year()
				article.Month = t2.Month()
			}
		}
	}

	if article.Excerpt == "" {
		article.Excerpt = truncateText(body, 200)
	}

	if article.Slug == "" {
		article.Slug = slugify(article.Title)
	}

	return article, nil
}

func renderMarkdownToHTML(md string) string {
	html := md

	html = regexp.MustCompile(`### (.+)`).ReplaceAllString(html, `<h3>$1</h3>`)
	html = regexp.MustCompile(`## (.+)`).ReplaceAllString(html, `<h2>$1</h2>`)
	html = regexp.MustCompile(`# (.+)`).ReplaceAllString(html, `<h1>$1</h1>`)

	html = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(html, `<strong>$1</strong>`)
	html = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(html, `<em>$1</em>`)

	html = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`).ReplaceAllString(html, `<img src="$2" alt="$1" loading="lazy">`)
	html = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(html, `<a href="$2">$1</a>`)

	html = regexp.MustCompile(`(?m)^- (.+)$`).ReplaceAllString(html, `<li>$1</li>`)

	html = regexp.MustCompile(`(?m)^\*\s(.+)$`).ReplaceAllString(html, `<li>$1</li>`)

	paragraphs := strings.Split(html, "\n\n")
	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "<h") || strings.HasPrefix(p, "<li") || strings.HasPrefix(p, "<img") {
			result = append(result, p)
		} else {
			result = append(result, "<p>"+p+"</p>")
		}
	}

	html = strings.Join(result, "\n")

	html = strings.ReplaceAll(html, "<li>", "<ul><li>")
	html = strings.ReplaceAll(html, "</li>", "</li></ul>")
	html = regexp.MustCompile(`</ul>\s*<ul>`).ReplaceAllString(html, "")

	return html
}

func countWords(s string) int {
	var count int
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			count++
			inWord = true
		}
	}
	return count
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	re := regexp.MustCompile(`[^a-z0-9\s-]`)
	s = re.ReplaceAllString(s, "")
	re = regexp.MustCompile(`[\s-]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func (s *Store) GetArticlesByYear() map[int][]models.Article {
	result := make(map[int][]models.Article)
	for _, a := range s.Articles {
		result[a.Year] = append(result[a.Year], a)
	}
	for year := range result {
		sort.Slice(result[year], func(i, j int) bool {
			di, _ := time.Parse("2006-01-02", result[year][i].Date)
			dj, _ := time.Parse("2006-01-02", result[year][j].Date)
			return di.After(dj)
		})
	}
	return result
}

func (s *Store) GetIssues() []models.Issue {
	issueMap := make(map[string]*models.Issue)
	var issueKeys []string

	for _, a := range s.Articles {
		t, err := time.Parse("2006-01-02", a.Date)
		if err != nil {
			continue
		}
		year := t.Year()
		quarter := int(t.Month()-1)/3 + 1
		key := fmt.Sprintf("%d-Q%d", year, quarter)

		if _, ok := issueMap[key]; !ok {
			quarters := map[int]string{1: "Spring", 2: "Summer", 3: "Autumn", 4: "Winter"}
			issueMap[key] = &models.Issue{
				Year:    year,
				Quarter: quarter,
				Label:   fmt.Sprintf("%s %d", quarters[quarter], year),
			}
			issueKeys = append(issueKeys, key)
		}
		issueMap[key].Articles = append(issueMap[key].Articles, a)
	}

	sort.Slice(issueKeys, func(i, j int) bool {
		return issueKeys[i] > issueKeys[j]
	})

	var issues []models.Issue
	for _, key := range issueKeys {
		issues = append(issues, *issueMap[key])
	}
	return issues
}

func (s *Store) GetArticle(slug string) (*models.Article, bool) {
	a, ok := s.BySlug[slug]
	if !ok {
		for i := range s.Articles {
			if s.Articles[i].Slug == slug {
				return &s.Articles[i], true
			}
		}
		return nil, false
	}
	return a, true
}

func (s *Store) GetLatestArticles(n int) []models.Article {
	if n > len(s.Articles) {
		n = len(s.Articles)
	}
	return s.Articles[:n]
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func (s *Store) GetArticlesByIssue(year, quarter int) []models.Article {
	var result []models.Article
	for _, a := range s.Articles {
		t, err := time.Parse("2006-01-02", a.Date)
		if err != nil {
			continue
		}
		if t.Year() == year && int(t.Month()-1)/3+1 == quarter {
			result = append(result, a)
		}
	}
	return result
}

func (s *Store) RecentArticles(n int) []models.Article {
	if n > len(s.Articles) {
		n = len(s.Articles)
	}
	return s.Articles[:n]
}

var _ = template.HTMLEscaper
