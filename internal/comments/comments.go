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
	mu          sync.RWMutex
	dataDir     string
	commentCache map[string]*models.ArticleComments
	likeCache    map[string]*models.ArticleLikes
}

func New(dataDir string) (*Manager, error) {
	m := &Manager{
		dataDir:      dataDir,
		commentCache: make(map[string]*models.ArticleComments),
		likeCache:    make(map[string]*models.ArticleLikes),
	}

	if err := os.MkdirAll(filepath.Join(dataDir, "comments"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "likes"), 0755); err != nil {
		return nil, err
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

func sanitizeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
