# Configuring Charm

Charm is configured by placing a TOML file at `/secret/charm.conf`

The file **must** specify values for:

* `Upstream` String the upstream host being stabilized
* `ReqFanFactor` Int how many duplicate requests to proxy to the upstream per request
* `TimeoutMS` Int number of milliseconds to wait for a good response from the upstream
* `MemcacheHosts` [String] list of memcache hosts
* `CacheSeconds` Int how many seconds to keep responses cached

## example

```
Upstream = "http://backend-api"
ReqFanFactor = 5
TimeoutMS = 1500 # 1.5 seconds
MemcacheHosts = ["memcache0:80", "memcache1:80"]
MemcacheSeconds = 20
```

# Runtime Options

You may set runtime options (currently only one) using environment variables

```
CHARM_LOG_LEVEL=[Debug|Info|Warn|Error|Fatal|Panic]
```