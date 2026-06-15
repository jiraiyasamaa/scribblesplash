package comments

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"scribblesplash/internal/models"

	"github.com/google/uuid"
)

type Manager struct {
	mu           sync.RWMutex
	dataDir      string
	commentCache map[string]*models.ArticleComments
	likeCache    map[string]*models.ArticleLikes
	shareCache   map[string]int
}

func New(dataDir string) (*Manager, error) {
	m := &Manager{
		dataDir:      dataDir,
		commentCache: make(map[string]*models.ArticleComments),
		likeCache:    make(map[string]*models.ArticleLikes),
		shareCache:   make(map[string]int),
	}

	for _, d := range []string{"comments", "likes", "shares"} {
		if err := os.MkdirAll(filepath.Join(dataDir, d), 0755); err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (m *Manager) GetComments(slug string) ([]models.Comment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cached, ok := m.commentCache[slug]; ok {
		var approved []models.Comment
		for _, c := range cached.Comments {
			if c.Approved {
				approved = append(approved, c)
			}
		}
		if len(approved) > 0 || len(cached.Comments) == 0 {
			return approved, nil
		}
	}

	path := filepath.Join(m.dataDir, "comments", slug+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.commentCache[slug] = &models.ArticleComments{Slug: slug}
			return nil, nil
		}
		return nil, err
	}

	var ac models.ArticleComments
	if err := json.Unmarshal(data, &ac); err != nil {
		return nil, err
	}
	m.commentCache[slug] = &ac

	var approved []models.Comment
	for _, c := range ac.Comments {
		if c.Approved {
			approved = append(approved, c)
		}
	}
	return approved, nil
}

func (m *Manager) GetAllComments() ([]CommentWithArticle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dir := filepath.Join(m.dataDir, "comments")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []CommentWithArticle
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var ac models.ArticleComments
		if err := json.Unmarshal(data, &ac); err != nil {
			continue
		}
		for _, c := range ac.Comments {
			result = append(result, CommentWithArticle{
				Comment:     c,
				ArticleSlug: slug,
			})
		}
	}
	return result, nil
}

type CommentWithArticle struct {
	models.Comment
	ArticleSlug string `json:"article_slug"`
}

func (m *Manager) ApproveComment(slug, commentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ac, err := m.loadComments(slug)
	if err != nil {
		return err
	}
	for i := range ac.Comments {
		if ac.Comments[i].ID == commentID {
			ac.Comments[i].Approved = true
			return m.saveComments(slug, ac)
		}
	}
	return fmt.Errorf("comment %s not found", commentID)
}

func (m *Manager) RejectComment(slug, commentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ac, err := m.loadComments(slug)
	if err != nil {
		return err
	}
	for i := range ac.Comments {
		if ac.Comments[i].ID == commentID {
			ac.Comments = append(ac.Comments[:i], ac.Comments[i+1:]...)
			return m.saveComments(slug, ac)
		}
	}
	return fmt.Errorf("comment %s not found", commentID)
}

func (m *Manager) loadComments(slug string) (*models.ArticleComments, error) {
	if cached, ok := m.commentCache[slug]; ok {
		return cached, nil
	}
	path := filepath.Join(m.dataDir, "comments", slug+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ac models.ArticleComments
	if err := json.Unmarshal(data, &ac); err != nil {
		return nil, err
	}
	m.commentCache[slug] = &ac
	return &ac, nil
}

func (m *Manager) AddComment(slug, name, email, body string) (*models.Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	comment := models.Comment{
		ID:        uuid.NewString(),
		Name:      name,
		Email:     email,
		Body:      body,
		CreatedAt: time.Now(),
		Approved:  true,
	}

	if name == "" {
		comment.Name = "Anonymous"
	}

	body = strings.TrimSpace(body)
	if len(body) < 2 || len(body) > 5000 {
		return nil, fmt.Errorf("comment body must be between 2 and 5000 characters")
	}

	comment.Body = sanitizeHTML(body)
	comment.Name = sanitizeHTML(comment.Name)

	ac, exists := m.commentCache[slug]
	if !exists {
		path := filepath.Join(m.dataDir, "comments", slug+".json")
		if data, err := os.ReadFile(path); err == nil {
			var loaded models.ArticleComments
			if json.Unmarshal(data, &loaded) == nil {
				ac = &loaded
			}
		}
		if ac == nil {
			ac = &models.ArticleComments{Slug: slug}
		}
		m.commentCache[slug] = ac
	}

	ac.Comments = append(ac.Comments, comment)

	if err := m.saveComments(slug, ac); err != nil {
		return nil, err
	}

	return &comment, nil
}

func (m *Manager) saveComments(slug string, ac *models.ArticleComments) error {
	path := filepath.Join(m.dataDir, "comments", slug+".json")
	data, err := json.MarshalIndent(ac, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (m *Manager) GetLikes(slug string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cached, ok := m.likeCache[slug]; ok {
		return cached.Count, nil
	}

	path := filepath.Join(m.dataDir, "likes", slug+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.likeCache[slug] = &models.ArticleLikes{Slug: slug}
			return 0, nil
		}
		return 0, err
	}

	var al models.ArticleLikes
	if err := json.Unmarshal(data, &al); err != nil {
		return 0, err
	}
	m.likeCache[slug] = &al
	return al.Count, nil
}

func (m *Manager) GetCommentCount(slug string) int {
	comments, err := m.GetComments(slug)
	if err != nil {
		return 0
	}
	return len(comments)
}

func (m *Manager) AddLike(slug, ip string) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	al, exists := m.likeCache[slug]
	if !exists {
		path := filepath.Join(m.dataDir, "likes", slug+".json")
		if data, err := os.ReadFile(path); err == nil {
			var loaded models.ArticleLikes
			if json.Unmarshal(data, &loaded) == nil {
				al = &loaded
			}
		}
		if al == nil {
			al = &models.ArticleLikes{Slug: slug}
		}
		m.likeCache[slug] = al
	}

	for _, likedIP := range al.LikedIPs {
		if likedIP == ip {
			return al.Count, false, nil
		}
	}

	al.Count++
	al.LikedIPs = append(al.LikedIPs, ip)

	path := filepath.Join(m.dataDir, "likes", slug+".json")
	data, err := json.MarshalIndent(al, "", "  ")
	if err != nil {
		return 0, false, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, false, err
	}

	return al.Count, true, nil
}

func (m *Manager) HasLiked(slug, ip string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	al, ok := m.likeCache[slug]
	if !ok {
		return false
	}
	for _, likedIP := range al.LikedIPs {
		if likedIP == ip {
			return true
		}
	}
	return false
}

func (m *Manager) AddShare(slug string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.dataDir, "shares", slug+".json")
	count := 1
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &count)
		count++
	}
	data, _ := json.MarshalIndent(count, "", "  ")
	os.WriteFile(path, data, 0644)
	m.shareCache[slug] = count
	return count
}

func (m *Manager) GetShares(slug string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cached, ok := m.shareCache[slug]; ok {
		return cached
	}

	path := filepath.Join(m.dataDir, "shares", slug+".json")
	if data, err := os.ReadFile(path); err == nil {
		var count int
		if json.Unmarshal(data, &count) == nil {
			m.shareCache[slug] = count
			return count
		}
	}
	return 0
}

func sanitizeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
