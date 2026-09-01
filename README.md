# Model Fleet

Model Fleet is a small OpenAI-compatible HTTP proxy that routes logical model
names to physical models hosted by different providers. Applications call one
endpoint with a role such as `meal-planner/analyst`; Model Fleet resolves that
role to a configured deployment and forwards the request to Groq or Mistral.

At this stage, Model Fleet is a working proxy rather than an automatic router.
The Milestone 1 foundation is in place, but routing decisions still use the
first configured deployment; quota-aware selection, health-aware routing, and
automatic failover are not implemented yet.

The response remains OpenAI-compatible and includes headers describing the
routing decision:

- `X-Model-Fleet-Provider`
- `X-Model-Fleet-ModelId`
- `X-Model-Fleet-DeploymentId`

## Requirements

- Go 1.26.4 or later
- `GROQ_API_KEY`
- `MISTRAL_API_KEY`

Both API keys are currently required at startup because the embedded
configuration defines connections for both providers.

## Run the Router

Set the provider credentials and start the server:

```bash
export GROQ_API_KEY="..."
export MISTRAL_API_KEY="..."
go run ./cmd/router
```

The router listens on `http://localhost:4545`.

Send an OpenAI-compatible chat completion request using a logical model name:

```bash
curl --include http://localhost:4545/v1/chat/completions \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "meal-planner/analyst",
    "messages": [
      {"role": "user", "content": "Suggest a simple dinner."}
    ]
  }'
```

The response body reports the selected physical model in its standard `model`
field. The `X-Model-Fleet-*` headers identify the provider, physical model, and
deployment chosen by the router.

## Configuration

The router configuration is embedded from
[`cmd/router/config.yaml`](cmd/router/config.yaml) when the command is built.
Rebuild or restart with `go run` after changing it.

The configuration has three layers:

1. `provider_connections` defines reusable provider endpoints and names the
   environment variable containing each credential.
2. `deployments` assigns a physical model to a provider connection.
3. `routes` maps an application-facing logical model to an ordered list of
   deployments.

For example:

```yaml
provider_connections:
  groq-personal:
    provider: groq
    endpoint: https://api.groq.com/openai/v1
    credential_ref: GROQ_API_KEY
    quota_pool: groq-personal

deployments:
  groq-analyst:
    connection: groq-personal
    model: openai/gpt-oss-120b

routes:
  meal-planner/analyst:
    deployments:
      - groq-analyst
```

The current router selects the first deployment in the route. Multiple entries
can be configured, but automatic failover is not implemented yet.

## Supported API

Model Fleet currently exposes:

```text
POST /v1/chat/completions
```

The supported OpenAI-compatible request subset includes messages, function
tools, tool choice, temperature, JSON response format, and reasoning effort.
Streaming is not currently supported.

## Tests

Run the deterministic test suite:

```bash
go test ./...
```

Run the same race-enabled suite used by CI:

```bash
go test -race -count=1 ./...
```

Live provider connectivity tests are opt-in and consume provider quota:

```bash
MODEL_FLEET_LIVE_TESTS=1 \
GROQ_API_KEY="..." \
MISTRAL_API_KEY="..." \
go test -race -count=1 -v ./internal/acceptance_tests/provider/...
```

## License

Licensed under the [MIT License](LICENSE).
