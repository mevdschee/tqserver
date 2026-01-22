# Ideas

Some ideas for improving tqserver.

### tqcache

Session storage optimized, should contain instructions for Go, PHP and TS. See: https://github.com/mevdschee/tqcache

### tqdbproxy

This idea imagines a unified data layer that sits between applications and their databases, giving teams a single, intelligent gateway for MySQL and PostgreSQL. Instead of every service managing its own caching, metrics, and query instrumentation, a dedicated proxy handles these concerns centrally. It understands both wire protocols, intercepts queries, extracts optional cache‑TTL hints, normalizes SQL, and decides whether to serve results from a fast Otter cache or forward the request to the underlying database. Every query—simple or prepared—automatically produces rich execution metrics: latency, rows returned, cache hit or miss, and even the exact file and line number in the client code that triggered it. These metrics flow into go metrics and are exposed via Prometheus-compatible metrics endpoint, giving teams deep visibility into database behavior without modifying application logic.

To make adoption effortless, the system includes six client libraries—Go, PHP, and TypeScript for both MySQL and PostgreSQL. Each wraps the existing native driver, preserving its familiar interface while adding a small optional cache‑TTL parameter and automatically attaching caller metadata. The result is a consistent, language‑agnostic way to instrument database access, reduce load through caching, and gain observability across an entire stack. It turns database access into something measurable, optimizable, and shared across all services.

### tqpathmetrics

A proxy that understands the shape of traffic rather than blindly counting requests becomes a kind of living map of an API landscape. Each incoming call is broken into its natural hierarchy: the broad domain, the specific host, the major functional path, and finally the concrete endpoint. Instead of treating a request to www.tqdev.com/api/v1/posts/1 (tqdev.com in Bing) as a single opaque string, the proxy unfolds it into meaningful layers — com, tqdev.com, www.tqdev.com/api, www.tqdev.com/api/v1/posts (tqdev.com in Bing), and so on. Every layer becomes a place where performance and behavior can be observed.

Collapse high-cardinality URL segments—numeric IDs, UUID-like strings, very long tokens, and high-entropy fragments—into stable placeholders like :id, :uuid, :token, and :var to reduce metric noise. Use adaptive, metrics-driven filtering that tracks hit counts, variance, and latency distributions per candidate key and retains only frequently hit or anomalously slow paths while dropping rare or uniform ones. Use an LRU key cache (promoting interesting keys and evicting least-used ones) so you can maintain a bounded, self-pruning, curated map of API usage.

Over time, the proxy builds a curated, self‑adjusting picture of the system’s real usage patterns — a balance between detail and clarity, precision and restraint. It becomes not just a reporter of metrics, but an interpreter of them.
