# Semantic search

`gote search -s <query>` ranks notes by meaning instead of keywords. It needs an
embedding provider configured in the `semantic:` block of
`~/.config/gote/config.yaml`. An absent or empty block disables the feature.

```yaml
semantic:
  provider: openai            # openai | command
  base_url: https://api.openai.com/v1
  model: text-embedding-3-small
  dimensions: 0               # >0 is sent to the provider as "dimensions"
  api_key: sk-...             # exactly one of api_key / api_key_env / api_key_cmd (openai only)
  api_key_env: OPENAI_API_KEY
  api_key_cmd: "security find-generic-password -s gote -w"
  embed_cmd: ~/bin/my-embedder   # provider: command
  batch_size: 64
  exclude_tags: [private]
  price_per_mtok: 0.02        # optional, for cost estimates
```

| Key | Default | Notes |
|---|---|---|
| `provider` | none | Required when the block is non-empty. |
| `base_url` | `https://api.openai.com/v1` | Any OpenAI-compatible `/embeddings` endpoint. |
| `model` | `text-embedding-3-small` | Required for `command`. Vectors are cached per model. |
| `dimensions` | `0` | When `>0`, sent as `dimensions` and part of the cache identity. |
| `batch_size` | `64` | Texts per embedding request. |
| `exclude_tags` | none | Notes with any of these tags are never embedded. |
| `price_per_mtok` | none | USD per million tokens, used for estimates only. |

Config errors name the offending key, for example `semantic.api_key`.

## What is embedded

- Notes are split at every heading. Text before the first heading is its own
  chunk. Sections over 2,000 characters are split at blank lines, then hard
  split at 2,000.
- Each chunk is embedded with a small header (title, heading path, tags,
  aliases) so a chunk about "Setup" still knows which note it belongs to.
- Drawings contribute one chunk made of their text elements.
- `semantic: false` in a note's frontmatter, or a tag in `exclude_tags`,
  excludes it.
- Vectors are keyed by model and chunk hash. Editing one section re-embeds one
  chunk. Changing a note's tags or title re-embeds all of its chunks.
  Deleting a note prunes its vectors.

## API key options (openai)

Set exactly one.

1. `api_key` stores the key in the config file. gote prints a warning from
   `gote embed` and `gote search -s` if that file is readable by group or
   other. Run `chmod 600` on it.
2. `api_key_env` names an environment variable. The background server inherits
   its environment from the shell that spawned it, so a variable exported later
   or only in another terminal will not be visible to it.
3. `api_key_cmd` runs `sh -c` with a 10 second timeout and uses the trimmed
   output. It is read when needed and never stored. Prefer this one:
   it does not depend on the spawning shell, for example
   `security find-generic-password -s gote -w` on macOS or `pass show openai`.

Keys are never logged and are redacted from error messages.

## Commands

- `gote embed` brings embeddings up to date. It resumes where an interrupted
  run stopped, because each batch is committed on its own.
- `gote embed --yes` is required when the model or dimensions changed and the
  new model has no vectors yet. The refusal prints the chunk count, token
  estimate, and cost (when `price_per_mtok` is set). Tokens are estimated as
  characters / 4.
- `gote embed status` shows chunks, embedded, and pending counts.
- `gote search -s [-n N] <query>` returns the best chunk per note. `score` is
  the cosine similarity, `line` the chunk's starting line.

While `gote serve` runs, it re-embeds changed notes about two seconds after the
last change. Failures, including the model-change refusal, are written to
`server.log`; run `gote embed --yes` to proceed.

## Command adapter protocol

`provider: command` runs `sh -c "<embed_cmd>"` once per batch (60 second
timeout). It writes JSON to the command's stdin:

```json
{"model": "my-model", "texts": ["first text", "second text"]}
```

and expects JSON on stdout with one vector per text, in order:

```json
{"embeddings": [[0.1, 0.2], [0.3, 0.4]]}
```

A non-zero exit, invalid JSON, or a count mismatch is an error and includes the
first 500 bytes of stderr. Vectors may have any length but must be consistent
for a given model; gote normalizes them.

### Example adapter (Python, Cohere)

```python
#!/usr/bin/env python3
import json, os, sys, urllib.request

req = json.load(sys.stdin)
body = json.dumps({
    "model": req["model"],
    "texts": req["texts"],
    "input_type": "search_document",
    "embedding_types": ["float"],
}).encode()
r = urllib.request.Request(
    "https://api.cohere.com/v2/embed",
    data=body,
    headers={
        "Authorization": "Bearer " + os.environ["COHERE_API_KEY"],
        "Content-Type": "application/json",
    },
)
try:
    with urllib.request.urlopen(r, timeout=50) as resp:
        out = json.load(resp)
except Exception as e:
    print(f"cohere request failed: {e}", file=sys.stderr)
    sys.exit(1)
json.dump({"embeddings": out["embeddings"]["float"]}, sys.stdout)
```

```yaml
semantic:
  provider: command
  model: embed-english-v3.0
  embed_cmd: ~/bin/cohere-embed
```

Queries are embedded with the same adapter, so an adapter that distinguishes
query from document input types will treat both the same way.
