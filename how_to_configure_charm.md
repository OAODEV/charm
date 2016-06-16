# Configuring Charm

Charm is configured by placing a TOML file at `/secret/charm.conf`

The file **must** specify values for:

* `Upstream` String the upstream host being stabilized
* `ReqFanFactor` Int how many duplicate requests to proxy to the upstream per request
* `TimeoutMS` Int number of milliseconds to wait for a good response from the upstream

## example

```
Upstream = "http://backend-api"
ReqFanFactor = 5
TimeoutMS = 1500 # 1.5 seconds
```
