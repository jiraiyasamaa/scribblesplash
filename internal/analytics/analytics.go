package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Analytics struct {
	mu         sync.RWMutex
	dataPath   string
	TotalViews int            `json:"total_views"`
	DailyViews map[string]int `json:"daily_views"`
	PageViews  map[string]int `json:"page_views"`
}

type Stats struct {
	TotalViews int
	DailyViews map[string]int
	PageViews  []PageViewCount
	TodayViews int
}

type PageViewCount struct {
	Path  string
	Count int
}

func New(dataDir string) *Analytics {
	a := &Analytics{
		dataPath:   filepath.Join(dataDir, "analytics.json"),
		DailyViews: make(map[string]int),
		PageViews:  make(map[string]int),
	}
	data, err := os.ReadFile(a.dataPath)
	if err == nil {
		json.Unmarshal(data, a)
	}
	if a.DailyViews == nil {
		a.DailyViews = make(map[string]int)
	}
	if a.PageViews == nil {
		a.PageViews = make(map[string]int)
	}
	return a
}

func (a *Analytics) Track(path string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.TotalViews++
	today := time.Now().Format("2006-01-02")
	a.DailyViews[today]++
	a.PageViews[path]++

	data, _ := json.MarshalIndent(a, "", "  ")
	os.WriteFile(a.dataPath, data, 0644)
}

func (a *Analytics) GetStats() Stats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	today := time.Now().Format("2006-01-02")

	var pageViews []PageViewCount
	for path, count := range a.PageViews {
		pageViews = append(pageViews, PageViewCount{Path: path, Count: count})
	}
	sort.Slice(pageViews, func(i, j int) bool {
		return pageViews[i].Count > pageViews[j].Count
	})
	if len(pageViews) > 5 {
		pageViews = pageViews[:5]
	}

	return Stats{
		TotalViews: a.TotalViews,
		DailyViews: a.DailyViews,
		PageViews:  pageViews,
		TodayViews: a.DailyViews[today],
	}
}

func (a *Analytics) GetArticleViews(slug string) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.PageViews["/article/"+slug]
}
