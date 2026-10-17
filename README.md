# prosoche

Lambda function that posts one Stoic passage to Discord each morning.

Passages come from two US public-domain texts on Project Gutenberg, embedded in the binary:

- Marcus Aurelius, _Meditations_, George Long translation ([#15877](https://www.gutenberg.org/ebooks/15877)): 487
  sections across 12 books
- Epictetus, _Enchiridion_, T. W. Higginson translation ([#45109](https://www.gutenberg.org/ebooks/45109)): 51 chapters
  (this edition merges the usual chapters 52–53 into LI)

The Lambda posts `passages[days since 2026-10-19 % N]`. Ingest interleaves the two works evenly, about one Enchiridion
chapter every 10–11 days, with each work still in its own order; after all 527 it starts over. No state, no repeats
within a cycle. Passages longer than Discord's 2000-character limit are split across several posts.

# Setup

Reuses captain-hook's Discord bot: the Lambda reads `bot_token` from `/captain-hook-production/discord`, so captain-hook
must be deployed and its bot must be in the server that has the prosoche channel.

1. `just d`

# Config

All in `terraform/variables.tf` defaults; override with a gitignored `terraform/terraform.tfvars` if needed.

| Variable                | Default                                                       |
| ----------------------- | ------------------------------------------------------------- |
| `discord_channel_id`    | `1561019950376681542`                                         |
| `discord_ssm_parameter` | `/captain-hook-production/discord` (only `bot_token` is read) |
| `schedules`             | weekdays 7:40 AM, weekends 11:00 AM                           |
| `timezone`              | `America/New_York` (EventBridge Scheduler handles DST)        |

For example, to change the send times:

```hcl
schedules = {
  weekday = "cron(30 6 ? * MON-FRI *)"
  weekend = "cron(0 9 ? * SAT,SUN *)"
}
```

# Usage

```bash
just preview 2026-10-20   # print that day's message, no network
just run                  # send today's passage from your machine (cp .env.template .env)
just invoke               # trigger the deployed Lambda for a real test send
just t                    # tests
```

# Ingest

`passages.json` is generated and committed. To rebuild it:

```bash
just ingest
```

This downloads both texts, strips the Gutenberg header, footer, and license, the translators' introductions, indexes,
and footnotes, as well as footnote markers (`[A]`, `[1]`, `+`) and Long's cross-references like `(iii. 7)`. It then
splits sections by number, writes `internal/passage/passages.json`, and prints a word-count histogram. Long's bracketed
supplied words like `[I learned]` are part of the translation and are kept.

Sections that only make sense together are merged via `meditationsMerges` in `cmd/ingest`: Meditations 1.1–4, 7.38–43,
11.30–32, and 11.37–38.

Rebuilding with a different passage count shifts which passage lands on which date.
