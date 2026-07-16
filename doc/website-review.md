# BreachLine Website Review

Review of the marketing site (`infra/website/src`, Hugo + custom `breachline-theme`) with a focus on giving it a more **premium feel**. Colour scheme intentionally left unchanged to match the BreachLine app (dark `#2E3436` base, green `#3aa666` accent).

## Summary

The site is clean and functional, but read as *generic dark template* rather than *premium product* for three reasons: a single rounded body font used for everything, no constrained reading measure (long lines), and flat panels with little typographic hierarchy. The changes below are typography- and polish-led - no palette change - and are already applied.

## Fonts - the biggest lever

**Before:** everything was Nunito, and only the **400** weight was actually loaded (a single `.woff2` pulled straight from `fonts.gstatic.com`). The CSS asked for weights 500/600/700 on the logo, nav, buttons and headings - none of which existed, so browsers **synthesised faux-bold** from the 400 face. That is a large part of why headings looked soft/generic.

**After - a two-font pairing** (loaded properly via Google Fonts with `preconnect` + `display=swap`):

| Role | Font | Why |
|------|------|-----|
| Headings, logo | **Space Grotesk** (500/600/700) | Geometric grotesk with distinctive detailing - reads modern and technical, fits a security/IR tool. Real bold weights. |
| Body, UI | **Inter** (400/500/600/700) | The de-facto product-UI typeface; superb legibility at small sizes, wide weight range. |

Pairing a distinctive display face with a neutral, highly-legible body face is the standard recipe for a premium feel. Space Grotesk gives the brand a bit of character; Inter keeps long-form content effortless to read.

*Alternative pairings considered (if you want to swap later):*
- **Editorial:** `Fraunces` (display serif) + `Inter` - more magazine/luxury, less "tool".
- **Ultra-clean:** `Sora` + `Inter` - softer, safer, less character than Space Grotesk.
- **Data-brand:** add `JetBrains Mono` as a mono accent for timestamps/log snippets - reinforces the "time series / logs" identity. Not applied yet; good candidate for the blog and feature copy.

## Other premium tweaks applied

- **Reading measure:** long-form prose (`article`/`.home` paragraphs and lists) capped at `72ch`. Full-bleed lines across the 1200px container were tiring to read; cards and grids stay full width.
- **Heading refinement:** tighter tracking (`-0.02em` on `h1`, `-0.01em` on `h2`) and tighter `line-height` - large display type looks intentional rather than default.
- **Font rendering:** `-webkit-font-smoothing: antialiased` + `text-rendering: optimizeLegibility` for cleaner glyphs on macOS/retina.
- **Nav interaction:** replaced the flat opacity hover with an animated accent underline (grows from the left) - a small, tasteful motion detail.
- **Selection colour:** text selection now uses the green accent tint instead of the OS default blue.
- **Accessibility polish:** `:focus-visible` outlines on links and buttons (keyboard nav was previously invisible); `scroll-behavior: smooth`.
- **Body line-height** nudged 1.6 → 1.65 for a more open, considered feel.

## Larger tweaks applied

These four were flagged as bigger design calls in the first pass and are now implemented (still no new hues - depth comes from shades of the existing dark and the existing green accent):

1. **Elevation / depth.** Content panels (`article`, `.home`) were the exact same colour as the page, so they didn't read as surfaces. They now sit one shade up (`#323A3C` vs the `#2E3436` page) with a hairline `1px solid rgba(255,255,255,0.06)` border, a soft shadow, and larger radius - the "premium depth" of a designed product, entirely within the same neutral family.
2. **Hero section.** The home page now opens with a proper hero: a large Space Grotesk headline, a one-line value proposition, primary (`Download`) + secondary (outline `Explore features`) CTAs, and the app screenshot in a framed, shadowed panel - two-column on desktop, stacked on mobile. Driven by `hero_heading` / `hero_subheading` / `hero_image` front-matter in `content/_index.md`, rendered by `layouts/index.html`. This is the biggest first-impression lift.
3. **Nav active state.** The current section is now highlighted (accent colour + persistent underline, `aria-current="page"`). Implemented with a robust prefix match on the page permalink rather than Hugo's `IsMenuCurrent`, which didn't match the URL-based menu entries - blog posts correctly highlight **Blog**.
4. **Mono accent for data.** `JetBrains Mono` now styles inline code (green pill), fenced code blocks, and blog timestamps - tying the site's typography to what the product does (logs, timestamps, queries).

**Incidental fix:** the app screenshot (`breachline-window.png`) existed only in the committed `public/` output, never in theme `static/` - a clean `hugo --cleanDestinationDir` would have deleted it. It's now a real source asset at `themes/breachline-theme/static/images/`.

## Recommendations not yet applied (optional)

- **Self-host fonts.** For reliability, privacy, and to avoid a third-party request/FOUT, consider self-hosting the three font families in `static/fonts/` rather than Google Fonts. Slightly more setup; more "premium infra."
- **Subtle background texture.** A very faint radial glow behind the header/hero (accent green at ~3% opacity) can add atmosphere while staying on-palette.

## Files changed

- `themes/breachline-theme/layouts/_default/baseof.html` - Google Fonts `preconnect` + stylesheet link (Inter + Space Grotesk + JetBrains Mono); nav active-state logic.
- `themes/breachline-theme/layouts/index.html` - hero section.
- `content/_index.md` - hero front-matter; removed the now-duplicated intro/image/CTA from the body.
- `themes/breachline-theme/static/css/style.css` - font stacks, weights, rendering, tracking, reading measure, nav underline/active, focus/selection, panel elevation, hero + secondary button, code/mono styling.
- `themes/breachline-theme/static/images/breachline-window.png` - screenshot promoted from build output to source asset.

No colours were changed. Rebuild with `hugo` (or `hugo server -D` for live preview).
