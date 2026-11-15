# Configuration

File: `$XDG_CONFIG_HOME/gote/config.yaml` (default `~/.config/gote/config.yaml`). Unknown keys are an error. `gote config` prints the file and `gote config-edit` opens it.

```yaml
notes_dir: ~/notes
editor: nvim
server: true
diagram_dir: diagrams
```

## Keys

| Key | Default | Meaning |
|---|---|---|
| `notes_dir` | required | Directory holding the notes. `~` is expanded. Must exist. |
| `editor` | `$EDITOR`, then `vi` | Command used to open notes. Arguments are allowed (`code -w`). |
| `server` | `true` | `false` runs every command in-process, as if `--no-server` were always set. |
| `diagram_dir` | `diagrams` | Directory, inside `notes_dir`, where new drawings are created. |
| `semantic` | absent (disabled) | Semantic search block, below. |

## `semantic`

Absent or empty disables semantic search. See [embedding.md](embedding.md) for setup.

```yaml
semantic:
  provider: openai
  base_url: https://api.openai.com/v1
  model: text-embedding-3-small
  api_key_cmd: "security find-generic-password -s gote -w"
  exclude_tags: [private]
```

| Key | Default | Meaning |
|---|---|---|
| `semantic.provider` | | `openai` (any OpenAI-compatible `/embeddings` endpoint) or `command`. |
| `semantic.base_url` | `https://api.openai.com/v1` | Endpoint root for `openai`. |
| `semantic.model` | | Embedding model name. Changing it re-embeds everything; `gote embed` asks for `--yes` first. |
| `semantic.dimensions` | `0` | When above 0, sent to the provider as `dimensions`. |
| `semantic.api_key` | | Literal key (`openai` only). Prefer the two options below; gote warns if the config file is group or world readable. |
| `semantic.api_key_env` | | Name of an environment variable holding the key. The server inherits the environment of the shell that spawned it. |
| `semantic.api_key_cmd` | | Shell command that prints the key; 10 second timeout. |
| `semantic.embed_cmd` | | Shell command for `provider: command`. Reads `{"model","texts"}` on stdin, writes `{"embeddings"}` to stdout. |
| `semantic.batch_size` | `64` | Texts per embedding request. |
| `semantic.exclude_tags` | `[]` | Notes with any of these tags are never embedded. A note can also opt out with `semantic: false` in its frontmatter. |
| `semantic.price_per_mtok` | | Price per million tokens, used only for cost estimates. |

For `openai`, set exactly one of `api_key`, `api_key_env`, `api_key_cmd`.

## Paths

| What | Where |
|---|---|
| Config | `$XDG_CONFIG_HOME/gote/config.yaml` |
| Templates | `$XDG_CONFIG_HOME/gote/templates/<name>.md` |
| Index, socket, server log | `$XDG_STATE_HOME/gote/` (default `~/.local/state/gote/`): `gote.db`, `gote.sock`, `server.log` |
| Trash | `<notes_dir>/.trash/` |
| Quick note | `<notes_dir>/.quick.md` |
