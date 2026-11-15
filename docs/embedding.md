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

## Supported providers

gote has two adapters:

| `provider` | Covers |
|---|---|
| `openai` (built in) | Any server that speaks the OpenAI `POST /embeddings` API. Set `base_url`, `model` and a key. |
| `command` | Anything else: a script you write that turns texts into vectors (see [Writing a command adapter](#writing-a-command-adapter)). |

Common `base_url` values for `provider: openai` (check the provider's docs for current model names):

| Provider | `base_url` | Key needed |
|---|---|---|
| OpenAI | `https://api.openai.com/v1` | yes |
| Google Gemini (OpenAI-compatible endpoint) | `https://generativelanguage.googleapis.com/v1beta/openai` | yes |
| Mistral | `https://api.mistral.ai/v1` | yes |
| Together AI | `https://api.together.xyz/v1` | yes |
| Ollama (local) | `http://localhost:11434/v1` | any non-empty value |
| LM Studio (local) | `http://localhost:1234/v1` | any non-empty value |
| llama.cpp server (local) | `http://localhost:8080/v1` | any non-empty value |

Providers with their own API shape (Cohere, Voyage, AWS Bedrock, Vertex AI,
an internal service) need a command adapter.

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
  run stopped, because each batch is committed on its own. On a vault that has
  never been embedded it refuses and prints the chunk count and estimate;
  rerun as `gote embed --yes` to start the first full embed.
- `gote embed --yes` is required for the first full embed, and again when the
  model or dimensions changed and the new model has no vectors yet. The
  refusal prints the chunk count, token
  estimate, and cost (when `price_per_mtok` is set). Tokens are estimated as
  characters / 4.
- `gote embed status` shows chunks, embedded, and pending counts.
- `gote search -s [-n N] <query>` returns the best chunk per note. `score` is
  the cosine similarity, `line` the chunk's starting line.

While `gote serve` runs, it re-embeds changed notes about two seconds after the
last change, but only in a vault that is already embedded. It never starts the
first full embed or a model-change re-embed. Those refusals are written to
`server.log`; run `gote embed --yes` to proceed.

## Writing a command adapter

### Protocol

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

### Rules for an adapter

- Read all of stdin (one JSON object), write exactly one JSON object to
  stdout, exit 0. Send logs and errors to stderr only: anything else on stdout
  breaks parsing.
- Return one vector per input text, in the same order. gote sends at most
  `batch_size` texts per call; lower `batch_size` if your provider limits
  request size or a batch can take close to the 60 second timeout.
- Every vector for a given `model` must have the same length. Changing the
  vector length means changing `model` (gote caches vectors per model and asks
  for `gote embed --yes` before re-embedding).
- `model` is passed through from the config. Use it, or ignore it if your
  adapter serves one model.
- Retries and rate limiting are the adapter's job. Exit non-zero to fail the
  batch; `gote embed` can be rerun and resumes where it stopped.
- The command runs through `sh -c`, so `~` and shell syntax work in
  `embed_cmd`. Make the script executable (`chmod +x`).
- Read API keys inside the adapter. The background server inherits its
  environment from the shell that spawned it, so prefer a keychain or password
  manager lookup over an environment variable (see API key options above).
- No key handling or network access is required: a fully local model works the
  same way.

### Test an adapter before configuring it

```sh
echo '{"model":"my-model","texts":["hello","world"]}' \
  | ~/bin/my-embedder \
  | jq '{count: (.embeddings | length), dims: (.embeddings[0] | length)}'
# {"count": 2, "dims": 1024}
```

Then add it to the config and run `gote embed status` (shows pending chunks),
`gote embed` (prints the size and cost of the first run), and
`gote embed --yes`.

### Example adapter: offline test embedder (Python, no dependencies)

Useful for trying semantic search without any provider. It hashes words into
a fixed-size vector, so it matches shared vocabulary rather than meaning.

```python
#!/usr/bin/env python3
import hashlib, json, re, sys

DIMS = 256
req = json.load(sys.stdin)
out = []
for text in req["texts"]:
    v = [0.0] * DIMS
    for word in re.findall(r"\w+", text.lower()):
        h = int(hashlib.sha1(word.encode()).hexdigest(), 16)
        v[h % DIMS] += 1.0
    out.append(v)
json.dump({"embeddings": out}, sys.stdout)
```

```yaml
semantic:
  provider: command
  model: hash-256
  embed_cmd: ~/bin/hash-embed
```

### Example adapter: Cohere (Python)

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
