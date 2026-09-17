# OpenAI-compatible chat completions

Clients call `POST /v1/chat/completions` with the same JSON shape as OpenAI Chat Completions.

## Authentication

`Authorization: Bearer <gateway-api-key>`

## Request

```json
{
  "model": "smart-chat",
  "messages": [{"role": "user", "content": "hello"}],
  "stream": false
}
```

`model` is a gateway alias (see `MODEL_ALIASES`), not necessarily an upstream model id.

## Non-stream response

OpenAI-shaped `chat.completion` with `choices[0].message.content`.

## Stream response

`Content-Type: text/event-stream`. Each line is `data: <json>` in OpenAI chunk format; stream ends with `data: [DONE]`.

## Errors

| Status | Meaning |
|--------|---------|
| 401 | Missing or invalid gateway key |
| 429 | Rate limit exceeded (per-key RPM) |
| 502 | All providers failed after fallback |
