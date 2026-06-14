# SCRIBBLESPLASH WEBSITE IMPLEMENTATION PLAN (WORDPRESS)

## 1. CURRENT ENVIRONMENT AUDIT
- **Platform:** WordPress.com (Hosted)
- **Current Layout:** Single-column chronological blog-roll.
- **Visual Style:** Minimalist, high-contrast, text-centric.
- **Content Assets:** 12 years of chronological articles (must be preserved).

## 2. THE TRANSFORMATION: FROM BLOG TO MAGAZINE
We will transition the site from a "Diary" feel to a "Media House" feel without deleting a single word of the past.

### 2.1 The Baskerville 2 Advantage
Baskerville 2 is a "Masonry" theme, which means it already likes to show posts in a grid. We are going to "supercharge" this grid to look like a professional magazine.

### 2.2 Custom "Magazine" CSS
You will paste this into **Appearance > Customize > Additional CSS**. This will make your titles bolder and give your "Pillar" categories distinct identities.

```css
/* Scribblesplash Magazine Enhancements */
.site-title {
    font-family: 'Playfair Display', serif;
    font-size: 3rem;
    text-transform: uppercase;
    letter-spacing: 2px;
}

.entry-title {
    font-weight: 800;
    line-height: 1.2;
}

/* Category Badges for Pillars */
.category-psychology { border-top: 4px solid #4A90E2; }
.category-culture { border-top: 4px solid #F5A623; }
.category-awareness { border-top: 4px solid #7ED321; }

/* The Legacy Archive Style */
.legacy-archive-section {
    background-color: #f9f9f9;
    padding: 40px;
    border-radius: 8px;
    margin-top: 50px;
}
```

---

## 4. PHASE 2: IMPLEMENTING THE "JOURNEY"
### The "Query Loop" Strategy
Instead of just showing "All Posts," we will use the **Query Loop Block** on a new Homepage:
1.  **Block 1:** Set to show only 1 post (the latest) in a large "Hero" format.
2.  **Block 2:** Set to show 6 posts from specific pillars in a 3-column grid.
3.  **Block 3 (The Legacy):** A Query Loop set to "Oldest First" or filtered for your earliest years (2014-2016).

---

## 5. YOUR FIRST TASK: THE "CODEX" LANDING PAGE
1.  Create a **New Page** called "Home".
2.  Go to `Settings > Reading` and set "Your homepage displays" to "A static page", then select "Home".
3.  In the "Home" page, paste the following **Custom HTML Block** to introduce your vision:

```html
<div style="text-align: center; padding: 50px 20px; background: #fff;">
    <h2 style="font-size: 2.5rem;">THE BORDERLESS WORLD</h2>
    <p style="font-size: 1.2rem; max-width: 800px; margin: 0 auto;">
        Founded in 2014, Scribblesplash is an anthropological movement dedicated to the 
        celebration of humanity as one species. Explore 12 years of writing that 
        evolved into a global vision.
    </p>
</div>
```
