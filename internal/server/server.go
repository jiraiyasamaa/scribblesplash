package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"scribblesplash/internal/comments"
	"scribblesplash/internal/models"
	"scribblesplash/internal/storage"
)

type Server struct {
	store        *storage.Store
	comments     *comments.Manager
	tmpl         *template.Template
	templatesDir string
	admin        *Admin
	RepoDir      string
}

func New(store *storage.Store, cm *comments.Manager, templatesDir string) (*Server, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"seq": func(start, end int) []int {
			var s []int
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}

	return &Server{
		store:        store,
		comments:     cm,
		tmpl:         tmpl,
		templatesDir: templatesDir,
		admin:        NewAdmin("articles"),
	}, nil
}

func (s *Server) getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /article/{slug}", s.handleArticle)
	mux.HandleFunc("GET /archive", s.handleArchive)
	mux.HandleFunc("GET /issue/{slug}", s.handleIssue)
	mux.HandleFunc("GET /about", s.handleAbout)

	mux.HandleFunc("POST /api/article/{slug}/like", s.handleLike)
	mux.HandleFunc("POST /api/article/{slug}/comment", s.handleComment)

	// Admin routes
	mux.HandleFunc("GET /admin/login", s.handleAdminLogin)
	mux.HandleFunc("POST /admin/login", s.handleAdminLogin)
	mux.HandleFunc("GET /admin", s.adminMiddleware(s.handleAdminDashboard))
	mux.HandleFunc("GET /admin/new", s.adminMiddleware(s.handleAdminNew))
	mux.HandleFunc("POST /admin/new", s.adminMiddleware(s.handleAdminNew))
	mux.HandleFunc("GET /admin/edit/{slug}", s.adminMiddleware(s.handleAdminEdit))
	mux.HandleFunc("POST /admin/edit/{slug}", s.adminMiddleware(s.handleAdminEdit))

	fs := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	return s.logMiddleware(s.securityHeaders(mux))
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) render(w http.ResponseWriter, pageFile string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := s.tmpl.Clone()
	if err != nil {
		log.Printf("error cloning template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	tmpl, err = tmpl.ParseFiles(filepath.Join(s.templatesDir, pageFile))
	if err != nil {
		log.Printf("error parsing template %s: %v", pageFile, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("error rendering template %s: %v", pageFile, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	recent := s.store.RecentArticles(8)
	issues := s.store.GetIssues()

	var currentIssue *struct {
		Year     int
		Quarter  int
		Label    string
		Articles []ArticleCard
	}

	if len(issues) > 0 {
		first := issues[0]
		cards := make([]ArticleCard, len(first.Articles))
		for i, a := range first.Articles {
			cards[i] = ArticleCard{
				Title:     a.Title,
				Slug:      a.Slug,
				Date:      a.FormattedDate(),
				Excerpt:   a.Excerpt,
				Category:  a.Category,
				ImageURL:  a.ImageURL,
				WordCount: a.WordCount,
			}
		}
		currentIssue = &struct {
			Year     int
			Quarter  int
			Label    string
			Articles []ArticleCard
		}{
			Year:     first.Year,
			Quarter:  first.Quarter,
			Label:    first.Label,
			Articles: cards,
		}
	}

	cards := make([]ArticleCard, len(recent))
	for i, a := range recent {
		cards[i] = ArticleCard{
			Title:     a.Title,
			Slug:      a.Slug,
			Date:      a.FormattedDate(),
			Excerpt:   a.Excerpt,
			Category:  a.Category,
			ImageURL:  a.ImageURL,
			WordCount: a.WordCount,
		}
	}

	data := struct {
		Title        string
		Description  string
		Articles     []ArticleCard
		CurrentIssue interface{}
	}{
		Title:        "Scribblesplash — The Borderless World",
		Description:  "An anthropological movement dedicated to the celebration of humanity as one species.",
		Articles:     cards,
		CurrentIssue: currentIssue,
	}

	s.render(w, "home.html", data)
}

func (s *Server) handleArticle(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	article, ok := s.store.GetArticle(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	comments, _ := s.comments.GetComments(slug)
	likeCount, _ := s.comments.GetLikes(slug)
	ip := s.getClientIP(r)
	liked := s.comments.HasLiked(slug, ip)

	data := struct {
		Title       string
		Article     struct {
			Title        string
			Slug         string
			Date         string
			Category     string
			ImageURL     string
			BodyHTML     template.HTML
			WordCount    int
			ReadingTime  int
		}
		Comments    []CommentCard
		LikeCount   int
		Liked       bool
		Description string
	}{
		Title:   article.Title + " — Scribblesplash",
		Description: article.Excerpt,
		Article: struct {
			Title        string
			Slug         string
			Date         string
			Category     string
			ImageURL     string
			BodyHTML     template.HTML
			WordCount    int
			ReadingTime  int
		}{
			Title:       article.Title,
			Slug:        article.Slug,
			Date:        article.FormattedDate(),
			Category:    article.Category,
			ImageURL:    article.ImageURL,
			BodyHTML:    template.HTML(article.BodyHTML),
			WordCount:   article.WordCount,
			ReadingTime: max(article.WordCount/200, 1),
		},
		Comments:  toCommentCards(comments),
		LikeCount: likeCount,
		Liked:     liked,
	}

	s.render(w, "article.html", data)
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	archive := s.store.GetArticlesByYear()
	years := make([]int, 0, len(archive))
	for y := range archive {
		years = append(years, y)
	}
	for i := 0; i < len(years); i++ {
		for j := i + 1; j < len(years); j++ {
			if years[i] < years[j] {
				years[i], years[j] = years[j], years[i]
			}
		}
	}

	archiveCards := make(map[int][]ArticleCard)
	for y, arts := range archive {
		cards := make([]ArticleCard, len(arts))
		for i, a := range arts {
			cards[i] = ArticleCard{
				Title:     a.Title,
				Slug:      a.Slug,
				Date:      a.FormattedDate(),
				Excerpt:   a.Excerpt,
				Category:  a.Category,
				ImageURL:  a.ImageURL,
				WordCount: a.WordCount,
			}
		}
		archiveCards[y] = cards
	}

	data := struct {
		Title       string
		Description string
		Years       []int
		Archive     map[int][]ArticleCard
	}{
		Title:       "Archive — Scribblesplash",
		Description: "All articles from 2015 to present.",
		Years:       years,
		Archive:     archiveCards,
	}

	s.render(w, "archive.html", data)
}

func (s *Server) handleIssue(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	parts := strings.SplitN(slug, "-", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	quarter, err := strconv.Atoi(parts[1])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	articles := s.store.GetArticlesByIssue(year, quarter)
	if len(articles) == 0 {
		http.NotFound(w, r)
		return
	}

	quarters := map[int]string{1: "Spring", 2: "Summer", 3: "Autumn", 4: "Winter"}
	label := fmt.Sprintf("%s %d", quarters[quarter], year)

	cards := make([]ArticleCard, len(articles))
	for i, a := range articles {
		cards[i] = ArticleCard{
			Title:     a.Title,
			Slug:      a.Slug,
			Date:      a.FormattedDate(),
			Excerpt:   a.Excerpt,
			Category:  a.Category,
			ImageURL:  a.ImageURL,
			WordCount: a.WordCount,
		}
	}

	data := struct {
		Title       string
		Description string
		IssueYear   int
		IssueQ      int
		IssueLabel  string
		Articles    []ArticleCard
	}{
		Title:       fmt.Sprintf("%s %d — Scribblesplash", label, year),
		Description: fmt.Sprintf("The %s %d issue of Scribblesplash.", label, year),
		IssueYear:   year,
		IssueQ:      quarter,
		IssueLabel:  label,
		Articles:    cards,
	}

	s.render(w, "issue.html", data)
}

func (s *Server) handleAbout(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title       string
		Description string
	}{
		Title:       "About — Scribblesplash",
		Description: "The Borderless World — an anthropological movement.",
	}
	s.render(w, "about.html", data)
}

func (s *Server) handleLike(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ip := s.getClientIP(r)

	count, added, err := s.comments.AddLike(slug, ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count":  count,
		"added":  added,
	})
}

func (s *Server) handleComment(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	body := strings.TrimSpace(r.FormValue("body"))

	if body == "" {
		http.Error(w, "Comment body is required", http.StatusBadRequest)
		return
	}

	comment, err := s.comments.AddComment(slug, name, email, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/article/"+slug+"#comments", http.StatusSeeOther)
	_ = comment
}

type ArticleCard struct {
	Title     string
	Slug      string
	Date      string
	Excerpt   string
	Category  string
	ImageURL  string
	WordCount int
}

type CommentCard struct {
	Name      string
	Body      string
	CreatedAt string
}

func toCommentCards(comments []models.Comment) []CommentCard {
	cards := make([]CommentCard, len(comments))
	for i, c := range comments {
		name := c.Name
		if name == "" {
			name = "Anonymous"
		}
		cards[i] = CommentCard{
			Name:      name,
			Body:      c.Body,
			CreatedAt: c.CreatedAt.Format("2 Jan 2006 at 15:04"),
		}
	}
	return cards
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
