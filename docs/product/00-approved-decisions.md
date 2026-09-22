# Approved Product Decisions

Status: Approved
Owner: Product owner (user)
Approval date: 2026-09-21 (updated by product owner deployment authorization)

## Product

Mini-Inference is a private-network/VPN appliance that serves the existing MiniCPM5-2B GGUF model through an authenticated OpenAI-compatible text API and an unauthenticated Simplified Chinese operations console.

## Users and operating posture

- API users are the owner's applications and development tools on a trusted LAN or VPN.
- The owner is the sole maintainer and operator.
- The service is intended to remain available as a household service, without a formal SLA.
- The current Apple Silicon Mac is the sole formal deployment and MVP verification environment.
- iOS is an approved API-client platform only; it does not host the service or run the model locally.

## Inference behavior

- Runtime: Docker Model Runner using llama.cpp and Metal on the current Mac.
- Model: `models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf`.
- Maximum context: 131072 input plus output tokens.
- Logical batch size: 2048.
- Default temperature: 0.8.
- One loaded model and exactly one active inference request.
- Reasoning is enabled by default and may be disabled per request.
- Chat responses expose reasoning separately as `reasoning_content`.
- Tool calls may be generated and returned; the platform never executes tools.
- Prompt/KV caching is enabled only for the lifetime of the loaded model.

## API contract

- LAN-facing inference port: 8888.
- Authentication: Bearer API key `888888`.
- Stable OpenAI-compatible text subset:
  - `GET /v1/models`
  - `POST /v1/chat/completions`
  - `POST /v1/completions`
  - streaming and non-streaming responses
- Unsupported parameters return an explicit 400 response.
- The platform does not promise full OpenAI compatibility.
- Advanced generation other than tool-call output is not part of the stable contract.
- Requests whose input plus requested output exceed 131072 tokens return 400 without truncation or summarization.

## Queue and lifecycle

- Bounded FIFO with 20 waiting requests and one active request.
- The 21st waiting request returns 429.
- A request waiting longer than 30 minutes returns 504 and is recorded as `queue_timeout`.
- The model is unloaded after platform startup.
- Requests received while the model is unloaded return 503 and do not enter the queue.
- Start loads and warms the model.
- Stop cancels the active request, clears the queue, and unloads the model.
- The console displays waiting requests and may cancel individual waiting requests; it cannot reorder them.
- Gateway or host restart marks active and waiting requests as `interrupted`; requests are not replayed.

## Console and observability

- The operations console uses Simplified Chinese.
- It has no login; any private-network visitor may view status, start/stop the model, and cancel waiting requests.
- It displays service/model state, queue state, CPU, unified memory, disk, Metal-enabled status, request/token metrics, throughput, time to first token, and request duration.
- Exact real-time Metal GPU utilization is not required.
- Alerts exist only in the console and structured logs; no email, IM, desktop, or webhook notifications.

## Data and recovery

- PostgreSQL persists request metadata and input/output/reasoning token counts across restarts.
- Prompt, response, reasoning, and tool argument bodies are never persisted or logged.
- Request metadata is retained for 30 days.
- Daily logical backups are stored under the project directory and retained for 7 days.
- Backup restoration must be demonstrated.

## Security decisions

- Docker Model Runner's unauthenticated API remains internal.
- A dedicated network-isolated control sidecar may hold Docker socket authority and expose only fixed status/load/unload operations.
- The owner accepts the private-network risks of the fixed weak API key, unauthenticated console, and privileged controller.
- Public internet exposure requires a new product/security decision and ADR.

## Explicit non-goals

- Chat UI.
- Multiple models.
- Public internet exposure.
- User accounts, roles, or multi-tenant accounting.
- High availability, clustering, or horizontal autoscaling.
- Tool execution.
- Embeddings, Anthropic, Ollama, or complete OpenAI API compatibility.
- Prompt or response history.
- Persistent KV cache across model unload or platform restart.
- Exact real-time Metal GPU utilization.
- Linux server deployment, compute-appliance deployment, and iOS local inference/server deployment in the MVP.

## Acceptance evidence

MVP completion requires real evidence for model load/warm/unload, authentication, streaming and non-streaming text generation, reasoning toggle and separation, tool-call output without execution, FIFO serialization, overload, timeout and cancellation, maximum context and overflow rejection, prompt cache reuse, monitoring, retention cleanup, backup/restore, restart interruption handling, and browser verification of the Chinese console. Performance is recorded as a reproducible baseline without a pass/fail threshold.
