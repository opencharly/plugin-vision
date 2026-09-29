# AGENTS.md — plugin-vision

Standalone plugin repo for the `vision:` check verb — validate an IMAGE (a
screenshot a check just produced) by sending it directly to an
OpenAI-compatible vision endpoint from charly. The plugin is a Go module at
`candy/plugin-vision/` (module path
`github.com/opencharly/plugin-vision/candy/plugin-vision`); the root `charly.yml`
declares `discover: candy` so the repo is a project and its candy is scanned.

Canonical files:

- `candy/plugin-vision/charly.yml` — the `plugin-vision:` candy entity
  (`plugin:` block, `plan:` checks).
- `candy/plugin-vision/plugin.go`, `provider.go`, `methods.go` — the provider and
  the verb handler.
- `candy/plugin-vision/schema/vision.cue` + `params/cue_types_gen.go` — the
  plugin's own `#VisionInput` schema and generated types.
- `candy/plugin-vision/cmd/serve/main.go` — the out-of-process serve shim.
- `.github/workflows/tag-on-merge.yml`.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the unified Provider model, the per-plugin CUE-schema contract,
  placement. Load before touching the provider or schema.
- `/charly-check:check` — the check-verb surface and the screenshot/recording
  verbs whose `artifact` path the `vision:` step reads.
- `/charly-internals:git-workflow` — before any git/PR action.

There is no dedicated `/charly-*:vision` owning skill — this repo's candy carries
no `skill:` entity. The gap is recorded against `opencharly/opencharly#291`;
when one is authored, add it here.

## Build / validate / test

- `cd candy/plugin-vision && go build ./...` — compile the plugin module.
- `cd candy/plugin-vision && go test ./...` — the hermetic unit + mock tests.
- `CHARLY_SPEC_SCHEMA=<spec>/schema/llm.cue go test ./...` — adds the schema
  parity check.
- `go test -run TestLive -v ./...` — live proof (needs an OpenAI-compatible
  endpoint; skips cleanly without one).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate and ships only `.github/workflows/tag-on-merge.yml`.

## Modify this repo

- Edit the `plugin-vision:` candy entity, the Go source, and `schema/vision.cue`
  **together** — the schema is the single source for the generated params.
- The endpoint config reuses the shared `#LLMSpec`/`#LLMParams` vocabulary and
  the SDK's shared `llmkit` client; keep resolution field-wise (`EVAL_LLM_*` >
  the step's `llm:` block > the built-in local-ollama default).
- The image is sent as a base64 data URL (ollama's OpenAI layer accepts base64
  data URLs only — remote image URLs are unsupported).

## Landing

Every change lands through a pull request gated by the org-required
`charly/pr-validator`. The landing mechanics — the `feat/` branch, the PR-only
rule, `CHANGELOG/` history, and the tag-on-merge CalVer — are owned by
`/charly-internals:git-workflow` and the umbrella `AGENTS.md` /
`charly/AGENTS.md`; this signpost points at them and does not restate them.
