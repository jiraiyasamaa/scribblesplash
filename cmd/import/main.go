package main

import (
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Items []Item `xml:"item"`
}

type Item struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	PubDate     string     `xml:"pubDate"`
	Creator     string     `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Description string     `xml:"description"`
	Content     string     `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Excerpt     string     `xml:"http://wordpress.org/export/1.2/excerpt/ encoded"`
	PostID      string     `xml:"http://wordpress.org/export/1.2/ post_id"`
	PostDate    string     `xml:"http://wordpress.org/export/1.2/ post_date"`
	PostName    string     `xml:"http://wordpress.org/export/1.2/ post_name"`
	Status      string     `xml:"http://wordpress.org/export/1.2/ status"`
	PostType    string     `xml:"http://wordpress.org/export/1.2/ post_type"`
	Categories  []Category `xml:"category"`
}

type Category struct {
	Domain   string `xml:"domain,attr"`
	Nicename string `xml:"nicename,attr"`
	Value    string `xml:",chardata"`
}

func main() {
	xmlPath := filepath.Join("archive", "website archive export", "yesterdayisashestomorrowwoodandonlytodaythefireshinesbright.WordPress.2026-05-14.xml")
	outputDir := "articles"

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output dir: %v\n", err)
		os.Exit(1)
	}

	data, err := os.ReadFile(xmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading XML: %v\n", err)
		os.Exit(1)
	}

	var rss RSS
	if err := xml.Unmarshal(data, &rss); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing XML: %v\n", err)
		os.Exit(1)
	}

	count := 0
	for _, item := range rss.Channel.Items {
		if item.PostType != "post" || item.Status != "publish" {
			continue
		}

		title := html.UnescapeString(item.Title)
		content := html.UnescapeString(item.Content)
		content = cleanContent(content)

		if content == "" {
			continue
		}

		date := parseDate(item.PostDate)
		slug := item.PostName
		if slug == "" {
			slug = slugify(title)
		}

		category := ""
		for _, cat := range item.Categories {
			if cat.Domain == "category" {
				category = html.UnescapeString(cat.Value)
				break
			}
		}

		year := "unknown"
		if t, err := time.Parse("2006-01-02 15:04:05", item.PostDate); err == nil {
			year = fmt.Sprintf("%d", t.Year())
		}

		yearDir := filepath.Join(outputDir, year)
		if err := os.MkdirAll(yearDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating year dir: %v\n", err)
			continue
		}

		excerpt := extractExcerpt(content, 200)

		frontmatter := fmt.Sprintf(`---
title: "%s"
slug: "%s"
date: "%s"
category: "%s"
image: ""
excerpt: "%s"
tags: []
---`, escapeYAML(title), escapeYAML(slug), date, escapeYAML(category), escapeYAML(excerpt))

		mdContent := frontmatter + "\n\n" + content

		filename := filepath.Join(yearDir, slug+".md")
		if err := os.WriteFile(filename, []byte(mdContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", filename, err)
			continue
		}

		count++
		fmt.Printf("Imported: %s (%s)\n", title, date)
	}

	fmt.Printf("\nImported %d posts successfully.\n", count)
}

func parseDate(dateStr string) string {
	t, err := time.Parse("2006-01-02 15:04:05", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("2006-01-02")
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

func cleanContent(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&#8211;", "–")
	s = strings.ReplaceAll(s, "&#8217;", "'")
	s = strings.ReplaceAll(s, "&#8220;", "\"")
	s = strings.ReplaceAll(s, "&#8221;", "\"")
	s = strings.ReplaceAll(s, "&#8230;", "...")
	s = strings.ReplaceAll(s, "&#038;", "&")

	s = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`</p>`).ReplaceAllString(s, "\n\n")
	s = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`<em[^>]*>(.*?)</em>`).ReplaceAllString(s, "*$1*")
	s = regexp.MustCompile(`<strong[^>]*>(.*?)</strong>`).ReplaceAllString(s, "**$1**")
	s = regexp.MustCompile(`<h1[^>]*>(.*?)</h1>`).ReplaceAllString(s, "# $1\n")
	s = regexp.MustCompile(`<h2[^>]*>(.*?)</h2>`).ReplaceAllString(s, "## $1\n")
	s = regexp.MustCompile(`<h3[^>]*>(.*?)</h3>`).ReplaceAllString(s, "### $1\n")
	s = regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`).ReplaceAllString(s, "[$2]($1)")
	s = regexp.MustCompile(`<img[^>]*src="([^"]*)"[^>]*alt="([^"]*)"[^>]*/>`).ReplaceAllString(s, "![$2]($1)")
	s = regexp.MustCompile(`<img[^>]*src="([^"]*)"[^>]*>`).ReplaceAllString(s, "![]($1)")
	s = regexp.MustCompile(`<ul[^>]*>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`</ul>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`<li[^>]*>(.*?)</li>`).ReplaceAllString(s, "- $1\n")
	s = regexp.MustCompile(`<blockquote[^>]*>(.*?)</blockquote>`).ReplaceAllString(s, "> $1\n")
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)
	return s
}

func extractExcerpt(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return strings.ReplaceAll(string(runes), "\"", "'")
	}
	truncated := string(runes[:maxLen])
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 {
		truncated = truncated[:lastSpace]
	}
	return strings.ReplaceAll(truncated, "\"", "'")
}

func escapeYAML(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return s
}
