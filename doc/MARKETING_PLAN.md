# BreachLine Marketing Plan

> A go-to-market and paid-acquisition plan for BreachLine, a cross-platform
> time-series analysis tool for cyber incident response. This document covers
> audience targeting, channel selection, platform pricing, and realistic
> expectations for ad budgets ranging from **$100 to $2,500 USD**.

---

## 1. What we are selling

BreachLine is a fast, cross-platform desktop tool for **visualizing and
analyzing time-series data (audit logs, security events) during incident
response investigations**. It loads multi-GB log files, provides an
SPL-like search language, timeline histograms, annotations, and workspaces.

**Business model (from `doc/PRICING.md`):**

| Product | Price | Role in funnel |
|---|---|---|
| Free application | $0 | Top-of-funnel, drives installs |
| Premium license (Workspaces + annotations) | $10/mo or $100/yr | Primary revenue |
| Workspace Sync (cloud + team collaboration) | $5/mo per 5 workspaces + $1/seat | Expansion / team revenue |

This is a **freemium** model. The free app is the marketing engine — the goal
of every dollar is to drive *qualified installs*, and let the product convert a
percentage of those to premium and, eventually, team sync subscriptions.

**Key implication for budget expectations:** with a ~$100/yr headline price and
free entry point, your realistic customer acquisition cost (CAC) target for a
solo/small operation is **$10–40 per paying user**. Small budgets should be
measured on *installs and email signups*, not immediate license sales.

---

## 2. Who we are marketing to (the niche)

BreachLine sits in the **DFIR (Digital Forensics & Incident Response)** niche —
a small, technical, English-first, globally distributed professional audience.
This is a blessing (cheap, targetable, community-driven) and a curse (small
total addressable market, ad-fatigued, skeptical of marketing).

### Primary personas

| Persona | Where they are | What they care about |
|---|---|---|
| **DFIR analyst / consultant** | Boutique IR firms, Big-4 cyber, MDR/MSSP | Speed on huge logs, timeline building, "supertimeline" workflows |
| **SOC / blue-team analyst** | Enterprise security teams | Fast triage, log pivoting, cheap tooling that isn't a SIEM |
| **Threat hunter / detection engineer** | Mature security orgs | Ad-hoc analysis outside the SIEM, exportable timelines |
| **Digital forensics examiner** | Law enforcement, e-discovery, legal | Defensible timelines, annotations, evidence handling |
| **Students / enthusiasts** | University, CTF, home labbers | Free tooling, learning DFIR, blogging their workflow |

### Where this niche actually pays attention

- **Reddit**: r/cybersecurity, r/blueteamsec, r/digitalforensics, r/computerforensics, r/AskNetsec, r/dfir
- **X/Twitter (#DFIR / #infosec)**: still the beating heart of the DFIR community
- **YouTube DFIR creators**: 13Cubed, John Hammond, DFIRScience, MyDFIR
- **Newsletters**: *This Week in 4n6*, *Digital Forensics Now*, *tl;dr sec*, *Detection Engineering Weekly*, *Risky Business News*
- **Communities**: DFIR Discord servers, the SANS DFIR community, BlueTeam/Blue Team Village Slack/Discord
- **Conferences**: SANS DFIR Summit, BSides (local), Black Hat Arsenal, Objective by the Sea
- **Discovery sites**: GitHub, Product Hunt, Hacker News ("Show HN"), r/netsec (for genuinely novel tools)

---

## 3. Do the free/organic work FIRST

Paid ads amplify a message that already resonates. Before (and alongside)
spending money, do the zero-cost work — for this niche it often outperforms
paid:

1. **Show HN / Product Hunt launch** — one-time spike of exactly the right
   technical audience. Free.
2. **r/DFIR, r/blueteamsec, r/digitalforensics organic posts** — "I built a
   free tool for X" posts do very well *if genuine and non-spammy*. Free.
3. **A short YouTube demo (3–5 min)** — DFIR buyers want to *see* speed on a
   10 GB file. This asset is reused in every paid channel.
4. **Comparison content** — blog posts like "BreachLine vs. Timeline Explorer",
   "Viewing Plaso/supertimeline output fast" capture high-intent search traffic
   organically.
5. **Engage creators** — offer 13Cubed/MyDFIR-tier creators a free look; an
   organic mention is worth more than most paid placements.

**Treat the paid budgets below as amplification on top of this foundation.**

---

## 4. Platform options & pricing for this niche

Prices are typical 2025–2026 ranges; treat as planning estimates, not quotes.
"Min viable" is the smallest spend at which a channel produces learnable data.

| Platform | Targeting fit for DFIR | Typical cost | Min viable test | Notes |
|---|---|---|---|---|
| **Reddit Ads** | ⭐⭐⭐⭐⭐ Subreddit-level targeting is perfect | CPC $0.30–1.50; CPM $2–6 | $100–150 | Best cheap channel. Target the subreddits in §2. Low CPCs, real technical eyeballs. |
| **X/Twitter Ads** | ⭐⭐⭐⭐ Keyword + follower-lookalike targeting | CPC $0.50–2.50; CPM $5–10 | $150 | DFIR community lives here. Target followers of DFIR figures/tools. |
| **Google Search Ads** | ⭐⭐⭐⭐ Captures high-intent queries | CPC $2–8 (security keywords are pricey) | $300 | Bid on "timeline analysis tool", "log timeline viewer", "supertimeline viewer", "DFIR timeline tool". Highest intent, best for license sales. |
| **YouTube / Google Display** | ⭐⭐⭐⭐ Placement-target DFIR channels | CPV $0.02–0.10; CPM $4–12 | $200 | Run your demo video as pre-roll on 13Cubed/John Hammond/DFIRScience. Great for a *visual* product. |
| **Newsletter sponsorship** | ⭐⭐⭐⭐⭐ Hand-picked niche audience | $150–600 per send (small/mid DFIR lists); $1k+ for tl;dr sec-tier | $150–400 | Highest-trust channel. *This Week in 4n6*, *Digital Forensics Now*, detection newsletters. Flat fee, no bidding. |
| **Podcast / YouTube creator shout-out** | ⭐⭐⭐⭐ | $200–1,500 depending on reach | $250 | Micro-creators (5–30k) are affordable and highly trusted. |
| **LinkedIn Ads** | ⭐⭐⭐ Precise job-title targeting, but expensive | CPC $8–15; CPM $30–60; ~$10/day min | $500 | Target titles: "Incident Responder", "SOC Analyst", "DFIR", "Forensic Analyst". Only worth it when pushing *team/sync* (higher LTV). |
| **Meta (FB/Instagram)** | ⭐ Poor B2B-security fit | CPC $0.50–2 | — | Generally skip. Audience/intent mismatch for DFIR. |
| **Conference / BSides sponsorship** | ⭐⭐⭐⭐ | $250–2,500 (BSides local → small booth) | $250 | A local BSides sponsorship is within budget and hits the community directly. |

---

## 5. Budget tiers & realistic expectations

Assumptions used for the funnel math (deliberately conservative):

- Blended paid CPC ≈ **$1.00** (weighted toward cheap Reddit/X, some Google).
- Landing-page → **install** conversion: **8–15%** of clicks (free product, low friction).
- Install → **paying** conversion: **2–5%** over time (freemium DFIR benchmark).
- Numbers are **ranges**; the low end is what to *expect*, the high end is a *good* outcome.

> These are honest planning numbers, not promises. A tiny niche means variance
> is high — one newsletter mention or a creator picking it up can outperform a
> month of ads.

### 💵 Tier 1 — $100 (Experiment / "does anyone care?")

- **Goal:** Validate messaging and landing page. Not to make sales.
- **Allocation:** 100% **Reddit Ads** to 3–4 DFIR subreddits.
- **Expected:** ~2,000–4,000 impressions · **70–130 clicks** · **8–18 installs** · **0–1 paying** (likely 0).
- **What you actually get:** Which headline/subreddit resonates, real click-through data, and a handful of first users. Treat this as *learning spend*.

### 💵 Tier 2 — $250 (First real test)

- **Allocation:** $150 Reddit + $100 X/Twitter (parallel A/B of channels).
- **Expected:** ~6,000–12,000 impressions · **180–320 clicks** · **20–45 installs** · **0–2 paying**.
- **What you get:** A channel comparison (Reddit vs X), a growing free-user base to gather feedback from, and 1–2 pieces of creative proven to work.

### 💵 Tier 3 — $500 (Validate a repeatable channel)

- **Allocation:** $200 Reddit + $100 X + $200 **newsletter sponsorship** (one mid-tier DFIR newsletter).
- **Expected:** ~15,000–30,000 impressions · **350–600 clicks** (plus a newsletter's trusted flat-reach) · **50–90 installs** · **1–4 paying**.
- **What you get:** First taste of the highest-ROI DFIR channel (newsletters). If the newsletter outperforms ads on cost-per-install, you've found your scaling lever.

### 💵 Tier 4 — $1,000 (Build momentum)

- **Allocation:** $250 Reddit · $150 X · $350 newsletter (one premium *or* two mid-tier) · $250 YouTube pre-roll on DFIR channels.
- **Expected:** ~40,000–80,000 impressions · **700–1,200 clicks** · **90–180 installs** · **3–8 paying**.
- **What you get:** Multi-channel presence during a "launch window," reinforcing the same message across Reddit + newsletter + video where your buyer already spends time. This is the first tier where compounding word-of-mouth becomes visible.

### 💵 Tier 5 — $2,500 (Serious push / launch campaign)

- **Allocation (example):**
  - $500 Reddit (scaled winning creative)
  - $300 X/Twitter
  - $500 Google Search (high-intent keyword capture)
  - $700 newsletters (one premium like *tl;dr sec*-tier + one DFIR-specific)
  - $250 YouTube pre-roll
  - $250 one micro-creator sponsorship *or* a local BSides sponsorship
- **Expected:** ~120,000–250,000 impressions · **1,800–3,500 clicks** · **220–450 installs** · **8–20 paying** (more over the following months as free users convert).
- **What you get:** A coordinated launch that saturates the DFIR niche for ~4–6 weeks, meaningful install volume for feedback and word-of-mouth, and enough conversion data to calculate a real CAC and decide whether to scale further.

### Summary table

| Budget | Primary channels | Est. clicks | Est. installs | Est. paying (near-term) | Best used for |
|---|---|---|---|---|---|
| $100 | Reddit | 70–130 | 8–18 | ~0 | Message validation |
| $250 | Reddit + X | 180–320 | 20–45 | 0–2 | Channel A/B test |
| $500 | Reddit + X + newsletter | 350–600 | 50–90 | 1–4 | Find scalable channel |
| $1,000 | + YouTube, premium newsletter | 700–1,200 | 90–180 | 3–8 | Build momentum |
| $2,500 | + Google Search, creator/BSides | 1,800–3,500 | 220–450 | 8–20 | Launch campaign |

---

## 6. Recommended sequence (how to actually spend)

Don't dump the whole budget at once. Spend in waves so each informs the next:

1. **Foundation (free):** Landing page with the demo video, Show HN + Product
   Hunt, organic Reddit posts. Instrument install + signup tracking.
2. **Wave 1 ($100–250):** Reddit + X. Find the winning headline and audience.
3. **Wave 2 ($250–500):** Add one newsletter sponsorship. Compare
   cost-per-install vs. ads.
4. **Wave 3 ($500–1,000):** Double down on the best channel; add YouTube
   pre-roll of the demo.
5. **Wave 4 ($1,000–2,500):** Coordinated launch across all winners + Google
   Search for high-intent capture. Time it with a product release or a
   conference (BSides/SANS DFIR Summit) for a natural news hook.

---

## 7. What to measure (KPIs)

Because this is freemium with delayed conversion, track the *funnel*, not just sales:

- **Cost per click (CPC)** — channel efficiency
- **Cost per install (CPI)** — the key early metric; aim for **< $5–8**
- **Install → premium conversion %** — measured over 30/60/90 days
- **Customer acquisition cost (CAC)** — target **< $40** against ~$100/yr LTV
- **Channel attribution** — use unique UTM links / download URLs per channel
- **Qualitative** — feedback from new free users; DFIR word-of-mouth is the real multiplier

**Rule of thumb:** if a channel delivers installs under ~$8 and any of those
convert to premium within 90 days, scale it. If CPI exceeds ~$15 with no
conversions, cut it.

---

## 8. Positioning & messaging notes

Lead with the pains this niche feels daily:

- **"Open 10 GB log files in seconds."** Speed on huge files is the hero.
- **"Splunk-style search, no SIEM required."** Familiar SPL-like syntax, zero infra.
- **"Build and annotate incident timelines fast."** Core IR workflow.
- **"Free to start. Cross-platform. No cloud required."** Removes adoption friction.

Save the premium/sync pitch (Workspaces, team collaboration) for *after* the
free app has proven itself — that's the upsell, not the hook. LinkedIn and
team-oriented messaging are where the higher-LTV sync subscription is worth
paying premium CPCs for.

---

## 9. Honest caveats

- **The TAM is small.** DFIR is a niche of a niche. Don't expect viral scale;
  expect a *loyal* base. Success looks like hundreds of engaged users, not
  hundreds of thousands.
- **Trust > ads.** This audience distrusts marketing. A respected creator or
  newsletter mention will beat any paid banner. Budget accordingly.
- **Conversion is slow.** Free users may run BreachLine for months before a
  real incident makes them want Workspaces. Judge campaigns on installs and
  engagement first, revenue second.
- **Numbers vary widely.** With small budgets in a small niche, a single lucky
  placement can 3× your results — or a dud creative can halve them. Spend in
  waves and let data steer.
