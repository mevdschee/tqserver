# Ideas

Some ideas for improving tqserver.

### tqcache

Session storage optimized, should contain instructions for Go, PHP and TS. See: https://github.com/mevdschee/tqcache

### tqdbproxy

This idea imagines a unified data layer that sits between applications and their databases, giving teams a single, intelligent gateway for MySQL and PostgreSQL. Instead of every service managing its own caching, metrics, and query instrumentation, a dedicated proxy handles these concerns centrally. It understands both wire protocols, intercepts queries, extracts optional cache‑TTL hints, normalizes SQL, and decides whether to serve results from a fast Otter cache or forward the request to the underlying database. Every query—simple or prepared—automatically produces rich execution metrics: latency, rows returned, cache hit or miss, and even the exact file and line number in the client code that triggered it. These metrics flow into go metrics and are exposed via Prometheus-compatible metrics endpoint, giving teams deep visibility into database behavior without modifying application logic.

To make adoption effortless, the system includes six client libraries—Go, PHP, and TypeScript for both MySQL and PostgreSQL. Each wraps the existing native driver, preserving its familiar interface while adding a small optional cache‑TTL parameter and automatically attaching caller metadata. The result is a consistent, language‑agnostic way to instrument database access, reduce load through caching, and gain observability across an entire stack. It turns database access into something measurable, optimizable, and shared across all services.

### tqapiproxy

A small-but-critical feature for any API proxy is full, human-friendly recording of every outbound and inbound API call. When your application calls HTTP endpoints you should keep a log of all calls and results in an easy-to-read format (for example a `.http` file that records request line, headers, body, response status, headers, body, timestamps and duration). When debugging an application it's essential to be able to see the exact API calls that were executed: the full request and response bodies, correlated IDs, and timing information.

 Implementation note: one practical approach is to route outbound traffic through a SOCKS5 proxy with DNS resolution performed over the SOCKS connection, optionally allowing injection of a custom CA for TLS inspection in development/tracing modes. Make this an explicit, configurable mode with clear opt-in, sampling and redaction controls so production privacy and TLS expectations are preserved.
 
 Be mindful of privacy and production safety: capture request/response bodies by default for development, but in production provide configurable redaction, sampling, and retention policies so PII/credentials are not stored unintentionally. Also expose metrics to Prometheus (or similar) so teams can alert on spikes in 5xx rates or latency regressions per hostname.


### tqpathmetrics

A proxy that understands the shape of traffic rather than blindly counting requests becomes a kind of living map of an API landscape. Each incoming call is broken into its natural hierarchy: the broad domain, the specific host, the major functional path, and finally the concrete endpoint. Instead of treating a request to www.tqdev.com/api/v1/posts/1 (tqdev.com in Bing) as a single opaque string, the proxy unfolds it into meaningful layers — com, tqdev.com, www.tqdev.com/api, www.tqdev.com/api/v1/posts (tqdev.com in Bing), and so on. Every layer becomes a place where performance and behavior can be observed.

Collapse high-cardinality URL segments—numeric IDs, UUID-like strings, very long tokens, and high-entropy fragments—into stable placeholders like :id, :uuid, :token, and :var to reduce metric noise. Use adaptive, metrics-driven filtering that tracks hit counts, variance, and latency distributions per candidate key and retains only frequently hit or anomalously slow paths while dropping rare or uniform ones. Use an LRU key cache (promoting interesting keys and evicting least-used ones) so you can maintain a bounded, self-pruning, curated map of API usage.

Over time, the proxy builds a curated, self‑adjusting picture of the system’s real usage patterns — a balance between detail and clarity, precision and restraint. It becomes not just a reporter of metrics, but an interpreter of them.

### worker store

The worker store is a small curated registry of approved worker repositories that teams can quickly git-clone into projects. Each entry records a friendly name, an approved git URL, a category (dev-support, example-projects, observability), a short description, and optional tags indicating runtime or production readiness. Dev-support workers (adminer, debuggers) are explicitly for development and default to safe, non-destructive settings; example-projects provide minimal, language-specific templates (PHP, Go, TypeScript) with README-driven integration guidance; observability workers expose or forward metrics and traces. The index is a simple YAML/JSON list consumable by CI, tooling, or a lightweight CLI for listing, searching, and cloning. Entries should be reviewed before inclusion; the index can live in docs or be served by a small validation service.

### tqserver cli

The `tqserver` CLI is an extensible, developer-focused command tool exposing the full framework surface similar to Symfony's console and Laravel's Artisan. It unifies project scaffolding, local dev servers, generator commands, observability helpers, database/queue utilities and comprehensive cache management (clear, warm, inspect, prune). The CLI includes `worker` subcommands to list, search, approve and clone entries from the curated worker store, and `config:check`/`config:validate` commands that run schema and environment consistency checks returning structured validation errors. Designed for both interactive and scriptable CI usage, it supports environment-aware operations, safe dev-only flags for admin workers, and a plugin API so teams can add custom commands and integrate tooling into existing workflows.

