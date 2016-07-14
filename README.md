# Charm
A stabilizing reverse proxy or a group of hummingbirds.

Deploy it in front of an async backend service to bend the curve toward the
faster responses.

# Performance Profiling

This project uses `net/http/pprof` and therefor provides a performance report at `/debug/pprof/`

Be aware of this when deploying. It is theoritically possible to expose secrets in stack straces (or the heap or something else) when making this accessable to the internet. It is currently designed to only be exposed internally to the cluster.
