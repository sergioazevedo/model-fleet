# Model Fleet

> An eval-certified, multi-provider LLM router with automated model lifecycle
> management.

Model Fleet is an experimental Go project for routing LLM requests across
independent providers while preserving application-specific behavioral
guarantees.

## Goals

- Fail over across independent provider quotas.
- Encapsulate provider differences behind stateless clients.
- Certify model deployments for application roles through application-owned
  evaluations.
- Monitor model availability and deprecations.
- Automate maintenance through bounded, auditable workflows.

## Status

Milestone 1 development. The Groq and Mistral provider clients, shared
OpenAI-compatible transport, and static provider-connection and route
configuration are complete. The current focus is the logical-role HTTP API.

## License

Licensed under the [MIT License](LICENSE).
