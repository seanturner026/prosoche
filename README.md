# prosoche

Lambda function that posts one Stoic passage to Discord each morning.

Passages come from two US public-domain texts on Project Gutenberg, embedded in the binary:

- Marcus Aurelius, _Meditations_, George Long translation ([#15877](https://www.gutenberg.org/ebooks/15877)): 487 sections
  across 12 books
- Epictetus, _Enchiridion_, T. W. Higginson translation ([#45109](https://www.gutenberg.org/ebooks/45109)): 51
  chapters (this edition merges the usual chapters 52–53 into LI)

The Lambda posts `passages[days since 2026-10-19 % N]`, in file order: Meditations 1.1 → 12.36, then Enchiridion 1 → 51,
then it starts over. No state, no repeats within a cycle. Passages longer than Discord's 2000-character limit are split
across several posts.

# Setup

1. Create a Discord channel and a bot with the `Send Messages` permission in it (see
   https://discord.com/developers/applications)
1. Copy the channel ID and bot token
1. Create `terraform/var.<tier>.tfvars` (gitignored):

   ```hcl
   discord_channel_id = "..."
   discord_bot_token  = "..."
   tier               = "staging"
   ```

1. `just workspace staging`
1. `just d`

# Config

| What                   | Where                                                                                    |
| ---------------------- | ---------------------------------------------------------------------------------------- |
| Discord channel, token | SSM SecureString `/prosoche-<tier>/discord` (JSON `channel_id`, `bot_token`), write-only |
| Send times             | `schedules` variable, default weekdays 7:40 AM, weekends 11:00 AM                        |
| Time zone              | `timezone` variable, default `America/New_York` (EventBridge Scheduler handles DST)      |

To change the send times, override `schedules` in your tfvars:

```hcl
schedules = {
  weekday = "cron(30 6 ? * MON-FRI *)"
  weekend = "cron(0 9 ? * SAT,SUN *)"
}
```

To rotate Discord credentials, update the tfvars, bump `value_wo_version` in `terraform/ssm.tf`, and `just d`.

# Usage

```bash
just preview 2026-10-20   # print that day's message, no network
just run                  # send today's passage from your machine (needs .env, see .env.template)
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

Sections that only make sense together are merged via `meditationsMerges` in `cmd/ingest`. Currently that's only
Meditations 1.1–4, the family list whose verb appears only in 1.1.

Rebuilding with a different passage count shifts which passage lands on which date.
