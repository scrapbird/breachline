---
title: "Welcome to the BreachLine Blog"
date: 2025-01-15T00:00:00Z
draft: false
summary: "Introducing the BreachLine blog - where we'll share release notes, investigation tips, and product news."
---

Welcome to the BreachLine blog. This is where we'll post release notes, deep dives on incident response workflows, and news about the product.

To add a new post, drop a markdown file into `content/blog/` and rebuild the site. See the notes below for the front matter fields we use.

## Adding a post

Create a file like `content/blog/my-post.md` with front matter:

```markdown
---
title: "My Post Title"
date: 2026-07-15T00:00:00Z
draft: false
summary: "One-line teaser shown on the blog index."
---

Your post body, written in **markdown**.
```

Set `draft: true` while writing - drafts are excluded from the built site unless you run `hugo server -D`. Posts are automatically sorted newest-first on the [blog index](/blog/).
