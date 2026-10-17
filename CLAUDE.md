# prosoche

Lambda that posts one Stoic passage a day to Discord. See README.md for setup and config.

## Architecture

- **Runtime**: Go 1.27 (stdlib `encoding/json/v2`), AWS Lambda (arm64, `provided.al2023`)
- **Trigger**: EventBridge Scheduler in `America/New_York` — weekdays 7:40 AM, weekends 11:00 AM (`var.schedules`)
- **Discord**: reuses captain-hook's bot. The Lambda reads only `bot_token` from captain-hook's SSM parameter
  (`var.discord_ssm_parameter`); the channel ID is a plain env var. Don't name the SSM path in README.md.
- **Single deployment**: no staging/production tiers or Terraform workspaces.
- Ask before adding AWS services beyond Lambda, EventBridge Scheduler, and SSM.

## Layout

- `cmd/ingest` — downloads the Gutenberg texts and writes `internal/passage/passages.json`
- `internal/passage` — embeds `passages.json`; `ForDate` picks `passages[days since Start % N]`
- `cmd/daily` — the Lambda; `-preview YYYY-MM-DD` prints instead of sending. Delivery is behind `Sender`.

## passages.json

- Generated and committed. Never hand-edit; change `cmd/ingest` and run `just ingest`.
- Any change to passage count or order shifts which passage lands on which date. Mention it when it happens.
- Sources are fixed: George Long's Meditations (Gutenberg #15877, not #2680, which is Casaubon) and Higginson's
  Enchiridion (#45109). No modern translations or other authors without asking.
- Higginson's edition has 51 chapters, not the usual 53. Tests assert 51.
- Sections that read as fragments alone are merged via `meditationsMerges`; Enchiridion is spread evenly through the
  Meditations by `interleave`.
- No splitting passages across days. Long passages go out as several Discord posts on the same day (2000-char limit).

## Commands

```bash
just t                    # tests
just preview 2026-10-20   # print a day's message
just run                  # send today's passage locally (needs .env + AWS creds)
just ingest               # rebuild passages.json
just d                    # build + terraform apply
just invoke               # trigger the deployed Lambda
```
