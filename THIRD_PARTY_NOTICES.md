# Third-Party Notices

## OpenAI-compatible Chat Completions Adapter

`backend/llm/chatcompletions.go` is a focused implementation derived from the
OpenAI Chat Completions adapter in
[`achetronic/adk-utils-go`](https://github.com/achetronic/adk-utils-go), version
`v1.0.0`.

Copyright 2026 Alby Hernández <hola@achetronic.com>.

The referenced work is licensed under the Apache License, Version 2.0. The
OpsKeeper implementation keeps only the text, streaming, token usage and
function calling behavior required by its ADK model boundary. OpsKeeper does
not import or depend on the `achetronic/adk-utils-go` module.
