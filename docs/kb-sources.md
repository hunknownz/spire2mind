# KB Source Plan

This repository now includes a small external knowledge harvester at `cmd/spire2kb`.

## What can be collected

- English Slay the Spire 2 knowledge pages from `slaythespire.wiki.gg` through the MediaWiki API.
- English Slay the Spire 1 overview pages from the same wiki as a broader reference layer.
- Official Slay the Spire 2 Steam store and news pages in English and Simplified Chinese.

## What cannot be trusted as public-web match data

- No stable public web API for full Slay the Spire 2 run telemetry was confirmed.
- Public web sources are good for cards, relics, monsters, events, mechanics, and patch notes.
- Public web sources are not a reliable source of authoritative per-turn or per-run match logs.

## Where match data should come from instead

- Live local game state: `GET /state` and `GET /events/stream` from the Bridge.
- Local run artifacts: `scratch/agent-runs/`.
- Aggregated local knowledge: `scratch/guidebook/living-codex.json`, `scratch/guidebook/guidebook.md`, and per-run `run-index.sqlite`.

## Output format

The harvester writes two files:

- `data/kb/external-docs.json`
- `data/kb/source-inventory.json`

`external-docs.json` is a normalized document corpus with:

- source metadata
- language
- source URL
- cleaned plain text

This format is suitable for:

- RAG ingestion
- offline embedding jobs
- later schema-specific extraction into `data/eng/*.json`

## Usage

```powershell
go run ./cmd/spire2kb -wiki-limit 40
```

Useful flags:

- `-output`
- `-inventory`
- `-wiki-limit`
- `-include-sts1-core`

## Recommended next step

Use this corpus as a staging layer, then add schema-specific extractors for:

- cards
- relics
- monsters
- events
- keywords

That keeps raw-source collection separate from high-trust canonical game data.
