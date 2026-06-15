package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"scribblesplash/internal/analytics"
	"scribblesplash/internal/dungeon"
)

type Admin struct {
	passwordHash string
	sessions     map[string]time.Time
	mu           sync.RWMutex
	storePath    string
}

func NewAdmin(storePath string) *Admin {
	pw := os.Getenv("ADMIN_PASSWORD")
	if pw == "" {
		pw = "scribblesplash"
	}
	h := sha256.Sum256([]byte(pw))
	return &Admin{
		passwordHash: hex.EncodeToString(h[:]),
		sessions:     make(map[string]time.Time),
		storePath:    storePath,
	}
}

func (a *Admin) generateToken() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(h[:])
}

func (a *Admin) auth(r *http.Request) bool {
	c, err := r.Cookie("admin_token")
	if err != nil {
		return false
	}
	a.mu.RLock()
	t, ok := a.sessions[c.Value]
	a.mu.RUnlock()
	if !ok || time.Since(t) > 24*time.Hour {
		return false
	}
	return true
}

func (a *Admin) login(password string) string {
	h := sha256.Sum256([]byte(password))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(a.passwordHash)) != 1 {
		return ""
	}
	token := a.generateToken()
	a.mu.Lock()
	a.sessions[token] = time.Now()
	a.mu.Unlock()
	return token
}

func (s *Server) adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.admin.auth(r) {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		password := r.FormValue("password")
		token := s.admin.login(password)
		if token == "" {
			s.renderAdmin(w, "admin-login.html", struct {
				Title string
				Error string
			}{Title: "Login — Scribblesplash Admin", Error: "Invalid password"})
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: "admin_token", Value: token, Path: "/admin",
			HttpOnly: true, SameSite: http.SameSiteLaxMode,
			MaxAge: 86400,
		})
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	s.renderAdmin(w, "admin-login.html", struct {
		Title string
		Error string
	}{Title: "Login — Scribblesplash Admin"})
}

func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	articles := s.store.Articles
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Date > articles[j].Date
	})
	type row struct {
		Title    string
		Slug     string
		Date     string
		Category string
		Views    int
		Likes    int
		Comments int
		Shares   int
	}
	rows := make([]row, len(articles))
	for i, a := range articles {
		likes, _ := s.comments.GetLikes(a.Slug)
		rows[i] = row{
			Title:    a.Title,
			Slug:     a.Slug,
			Date:     a.Date,
			Category: a.Category,
			Views:    s.analytics.GetArticleViews(a.Slug),
			Likes:    likes,
			Comments: s.comments.GetCommentCount(a.Slug),
			Shares:   s.comments.GetShares(a.Slug),
		}
	}

	stats := s.analytics.GetStats()
	dungeonCount := s.dungeon.Count()

	s.renderAdmin(w, "admin-dashboard.html", struct {
		Title        string
		Articles     []row
		DungeonCount int
		Stats        analytics.Stats
	}{
		Title:        "Dashboard — Scribblesplash Admin",
		Articles:     rows,
		DungeonCount: dungeonCount,
		Stats:        stats,
	})
}

func (s *Server) handleAdminNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		s.handleAdminSave(w, r, "")
		return
	}
	s.renderAdmin(w, "admin-edit.html", struct {
		Title        string
		ArticleTitle string
		Slug         string
		Date         string
		Category     string
		Excerpt      string
		Body         string
		Tags         string
		ImageURL     string
		AuthorName   string
		AuthorLink   string
		IsNew        bool
	}{Title: "New Article — Scribblesplash Admin", IsNew: true, Date: time.Now().Format("2006-01-02")})
}

func (s *Server) handleAdminEdit(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	article, ok := s.store.GetArticle(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == "POST" {
		s.handleAdminSave(w, r, slug)
		return
	}
	s.renderAdmin(w, "admin-edit.html", struct {
		Title        string
		ArticleTitle string
		Slug         string
		Date         string
		Category     string
		Excerpt      string
		Body         string
		Tags         string
		ImageURL     string
		AuthorName   string
		AuthorLink   string
		IsNew        bool
	}{
		Title:       "Edit — " + article.Title + " — Scribblesplash Admin",
		ArticleTitle: article.Title,
		Slug:        article.Slug,
		Date:        article.Date,
		Category:    article.Category,
		Excerpt:     article.Excerpt,
		Body:        article.Content,
		Tags:        strings.Join(article.Tags, ", "),
		ImageURL:    article.ImageURL,
		AuthorName:  article.AuthorName,
		AuthorLink:  article.AuthorLink,
	})
}

func (s *Server) handleAdminSave(w http.ResponseWriter, r *http.Request, existingSlug string) {
	title := strings.TrimSpace(r.FormValue("title"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	date := strings.TrimSpace(r.FormValue("date"))
	category := strings.TrimSpace(r.FormValue("category"))
	excerpt := strings.TrimSpace(r.FormValue("excerpt"))
	body := r.FormValue("body")
	tags := strings.TrimSpace(r.FormValue("tags"))
	imageURL := strings.TrimSpace(r.FormValue("image_url"))

	if title == "" || slug == "" || date == "" {
		http.Error(w, "Title, slug, and date are required", http.StatusBadRequest)
		return
	}

	slug = slugify(slug)

	tagList := []string{}
	if tags != "" {
		for _, t := range strings.Split(tags, ",") {
			tagList = append(tagList, strings.TrimSpace(t))
		}
	}

	writtenBySelf := r.FormValue("written_by_self") == "on"
	authorName := strings.TrimSpace(r.FormValue("author_name"))
	authorLink := strings.TrimSpace(r.FormValue("author_link"))
	if writtenBySelf {
		authorName = ""
		authorLink = ""
	}

	year := date[:4]
	dir := filepath.Join(s.admin.storePath, year)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "Error creating directory", http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`---
title: %q
slug: %q
date: %q
category: %q
image: %q
excerpt: %q
tags: %v`, title, slug, date, category, imageURL, excerpt, tagList))
	if authorName != "" {
		sb.WriteString(fmt.Sprintf(`
author_name: %q
author_link: %q`, authorName, authorLink))
	}
	sb.WriteString(fmt.Sprintf(`
---

%s`, body))

	content := sb.String()

	filePath := filepath.Join(dir, slug+".md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		http.Error(w, "Error writing file", http.StatusInternalServerError)
		return
	}

	// Delete old file if slug changed
	if existingSlug != "" && existingSlug != slug {
		oldPath := s.findArticleFile(existingSlug)
		if oldPath != "" {
			os.Remove(oldPath)
		}
	}

	// Reload store so new article is visible immediately
	if err := s.store.Reload(s.admin.storePath); err != nil {
		log.Printf("error reloading store: %v", err)
	}

	// Git commit and push
	go s.gitCommitPush(slug, title)

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) findArticleFile(slug string) string {
	entries, err := os.ReadDir(s.admin.storePath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		yearDir := filepath.Join(s.admin.storePath, entry.Name())
		files, err := os.ReadDir(yearDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() && strings.TrimSuffix(f.Name(), ".md") == slug {
				return filepath.Join(yearDir, f.Name())
			}
		}
	}
	return ""
}

func (s *Server) gitCommitPush(slug, title string) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Printf("GITHUB_TOKEN not set, skipping git push for %s", slug)
		return
	}

	repoURL := fmt.Sprintf("https://jiraiyasamaa:%s@github.com/jiraiyasamaa/scribblesplash.git", token)

	cmds := []struct {
		name string
		args []string
	}{
		{"git", []string{"config", "user.name", "The Waterman"}},
		{"git", []string{"config", "user.email", "jiraiyachan@protonmail.com"}},
		{"git", []string{"add", "-A"}},
		{"git", []string{"commit", "-m", fmt.Sprintf("Add article: %s", title), "--allow-empty"}},
		{"git", []string{"push", repoURL, "master"}},
	}

	for _, cmd := range cmds {
		c := exec.Command(cmd.name, cmd.args...)
		c.Dir = s.RepoDir
		if out, err := c.CombinedOutput(); err != nil {
			log.Printf("git %s error: %v: %s", cmd.args[0], err, string(out))
			return
		}
	}
	log.Printf("Successfully pushed article %s to GitHub", slug)
}

func (s *Server) renderAdmin(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmplPath := filepath.Join(s.templatesDir, tmpl)
	t, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Printf("error parsing admin template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := t.Execute(w, data); err != nil {
		log.Printf("error rendering admin template: %v", err)
	}
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('-')
		}
	}
	slug := b.String()
	slug = strings.Trim(slug, "-")
	slug = strings.ReplaceAll(slug, "--", "-")
	return slug
}

func (s *Server) handleAdminDelete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	title := slug
	if a, ok := s.store.GetArticle(slug); ok {
		title = a.Title
	}

	if err := s.dungeon.Delete(slug); err != nil {
		log.Printf("error deleting article %s: %v", slug, err)
		http.Error(w, "Error deleting article", http.StatusInternalServerError)
		return
	}
	if err := s.store.Reload(s.admin.storePath); err != nil {
		log.Printf("error reloading store: %v", err)
	}

	go s.gitCommitPush(slug, "Delete: "+title)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminDungeon(w http.ResponseWriter, r *http.Request) {
	entries, err := s.dungeon.List()
	if err != nil {
		log.Printf("error listing dungeon: %v", err)
		http.Error(w, "Error loading dungeon", http.StatusInternalServerError)
		return
	}
	s.renderAdmin(w, "admin-dungeon.html", struct {
		Title   string
		Entries []dungeon.Entry
	}{
		Title:   "Dungeon — Scribblesplash Admin",
		Entries: entries,
	})
}

func (s *Server) handleAdminDungeonRestore(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	title := slug
	entries, err := s.dungeon.List()
	if err == nil {
		for _, e := range entries {
			if e.Slug == slug {
				title = e.Title
				break
			}
		}
	}

	if err := s.dungeon.Restore(slug); err != nil {
		log.Printf("error restoring article %s: %v", slug, err)
		http.Error(w, "Error restoring article", http.StatusInternalServerError)
		return
	}
	if err := s.store.Reload(s.admin.storePath); err != nil {
		log.Printf("error reloading store: %v", err)
	}

	go s.gitCommitPush(slug, "Restore: "+title)
	http.Redirect(w, r, "/admin/dungeon", http.StatusSeeOther)
}

func (s *Server) handleAdminDungeonPermanentDelete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if err := s.dungeon.PermanentDelete(slug); err != nil {
		log.Printf("error permanently deleting article %s: %v", slug, err)
		http.Error(w, "Error deleting article", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/dungeon", http.StatusSeeOther)
}

func (s *Server) handleAdminComments(w http.ResponseWriter, r *http.Request) {
	allComments, err := s.comments.GetAllComments()
	if err != nil {
		log.Printf("error listing comments: %v", err)
		http.Error(w, "Error loading comments", http.StatusInternalServerError)
		return
	}

	type commentRow struct {
		ID          string
		ArticleSlug string
		ArticleName string
		Name        string
		Body        string
		Date        string
		Approved    bool
	}

	rows := make([]commentRow, 0, len(allComments))
	for _, c := range allComments {
		articleName := c.ArticleSlug
		if a, ok := s.store.GetArticle(c.ArticleSlug); ok {
			articleName = a.Title
		}
		rows = append(rows, commentRow{
			ID:          c.ID,
			ArticleSlug: c.ArticleSlug,
			ArticleName: articleName,
			Name:        c.Name,
			Body:        c.Body,
			Date:        c.CreatedAt.Format("2 Jan 2006 15:04"),
			Approved:    c.Approved,
		})
	}

	s.renderAdmin(w, "admin-comments.html", struct {
		Title    string
		Comments []commentRow
	}{
		Title:    "Comments — Scribblesplash Admin",
		Comments: rows,
	})
}

func (s *Server) handleAdminCommentApprove(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	id := r.PathValue("id")
	if err := s.comments.ApproveComment(slug, id); err != nil {
		log.Printf("error approving comment: %v", err)
	}
	http.Redirect(w, r, "/admin/comments", http.StatusSeeOther)
}

func (s *Server) handleAdminCommentReject(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	id := r.PathValue("id")
	if err := s.comments.RejectComment(slug, id); err != nil {
		log.Printf("error rejecting comment: %v", err)
	}
	http.Redirect(w, r, "/admin/comments", http.StatusSeeOther)
}
