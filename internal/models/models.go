package models

import "time"

type Article struct {
	Title     string   `yaml:"title"`
	Slug      string   `yaml:"slug"`
	Date      string   `yaml:"date"`
	Year      int
	Month     time.Month
	Category  string   `yaml:"category"`
	Tags      []string `yaml:"tags"`
	ImageURL  string   `yaml:"image"`
	Excerpt   string   `yaml:"excerpt"`
	Content   string   `yaml:"-"`
	WordCount int      `yaml:"-"`
	BodyHTML  string   `yaml:"-"`
}

func (a *Article) Quarter() int {
	return int(a.Month-1)/3 + 1
}

func (a *Article) QuarterLabel() string {
	quarters := map[int]string{1: "Spring", 2: "Summer", 3: "Autumn", 4: "Winter"}
	return quarters[a.Quarter()]
}

func (a *Article) FormattedDate() string {
	t, err := time.Parse("2006-01-02", a.Date)
	if err != nil {
		return a.Date
	}
	return t.Format("2 January 2006")
}

type Comment struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	Approved  bool      `json:"approved"`
}

type ArticleComments struct {
	Slug     string    `json:"slug"`
	Comments []Comment `json:"comments"`
}

type ArticleLikes struct {
	Slug      string   `json:"slug"`
	Count     int      `json:"count"`
	LikedIPs  []string `json:"liked_ips"`
}

type Issue struct {
	Year     int
	Quarter  int
	Label    string
	Articles []Article
}

type PageData struct {
	Title       string
	Description string
	Articles    []Article
	Article     *Article
	Issues      []Issue
	Issue       *Issue
	Archive     map[int][]Article
	Comments    []Comment
	LikeCount   int
	Liked       bool
	Year        int
}
