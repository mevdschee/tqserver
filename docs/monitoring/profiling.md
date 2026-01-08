# Profiling

*(Documentation In Progress)*

Profiling Go workers can be done using standard Go pprof tools. Ensure your worker imports `net/http/pprof` and exposes the debug endpoint, or use `runtime/pprof` to capture profiles to disk.
