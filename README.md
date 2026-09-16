# opencharly/plugin-vision

The `vision` check verb for [charly](https://github.com/opencharly/charly) —
validate an IMAGE (a screenshot a check just produced) by sending it directly to
an OpenAI-compatible vision endpoint.

## What it is

A standalone charly plugin candy (`candy/plugin-vision`). charly's loader fetches
this repo, go-builds the provider binary on the host, and serves it out-of-process
over go-plugin gRPC via the charly plugin SDK — so the `vision:` verb dispatches
through the provider registry exactly like a built-in. The same provider compiles
INTO charly in-process for a build that lists it in `compiled_plugins`.

## Usage (a `check:` step)

```yaml
- check: the screenshot shows the new panel layout
  vision:
    prompt: "Does this screenshot show a top panel with a clock on the right?"
    artifact: /tmp/shot.png          # the path a screenshot verb just wrote
    expect:
      contains: "yes"
  context: [runtime]
```

`artifact` is the SAME path convention the recording/screenshot verbs already use
(`wl:` / `vnc:` / `cdp:` / `spice: screenshot`), so a check reads the screenshot it
just captured with no new plumbing. The image is sent as a base64 data URL
(ollama's OpenAI layer accepts base64 data URLs only — remote image URLs are
unsupported). The model's reply IS the verb's stdout, so the step's `stdout:`
matchers grade it exactly like any other verb; `expect:` is a convenience form
(`contains` / `matches` / `not_contains`).

## Endpoint configuration

The step's `llm:` block reuses the shared `#LLMSpec`/`#LLMParams` vocabulary:

```yaml
  vision:
    prompt: "..."
    artifact: /tmp/shot.png
    llm:
      base_url: "http://localhost:11434/v1"
      model: "qwen3-vl:8b"
      api_key: "$env.OPENAI_API_KEY"
      params: {max_tokens: 300, reasoning_effort: "none"}
```

Resolution is field-wise: `EVAL_LLM_*` env > the step's `llm:` block > the
built-in local-ollama default. The client is the SDK's shared `llmkit` — the same
client the pipeline engine's agent stages use.

## Build & test

```sh
cd candy/plugin-vision
go test ./...                     # unit + mock (hermetic)
CHARLY_SPEC_SCHEMA=<spec>/schema/llm.cue go test ./...   # + the schema parity check
go test -run TestLive -v ./...    # live proof (needs an OpenAI-compatible endpoint)
```
