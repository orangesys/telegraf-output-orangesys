# README #

Telegraph plugin to push metrics on Orangesys

### Telegraph output for Orangesys ###

* Execute a post http on Warp10 at every flush time configured in telegraph in order to push the metrics collected

### Install ###

* Git clone / go get telegraph source files (https://github.com/influxdata/telegraf)

* In the telegraf main dir, add this plugin as git submodule
```
git submodule add -b master https://github.com/orangesys/telegraf-output-orangesys.git plugins/outputs/orangesys
```

* Add the plugin in the plugin list, you need to add this line to plugins/output/all/all.go
```
_ "github.com/influxdata/telegraf/plugins/outputs/orangesys"
```

* do the 'make' command

* Add following instruction in the config file (Output part)

```
[global_tags]

[agent]
  interval = "1m"
  round_interval = true
  metric_batch_size = 1000
  metric_buffer_limit = 10000
  collection_jitter = "0s"
  flush_interval = "1m"
  flush_jitter = "0s"
  precision = ""
  debug = true
  logfile = ""
  hostname = "orangesys-test-client"
  omit_hostname = false

[[outputs.orangesys]]
  urls = ["https://demo.i.orangesys.io"] # orangesys https endpoint example
  database = "telegraf" # required
  jwt_token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiI5ODZlMDU0MDFmY2I0MGM3YWU1ZDk5MzMxN2Y2NmE2MiJ9.8U_ApqEQqwp2Uaf69NnHMfVa4bhBGiOxlrF7x3o1O-8"

[[inputs.cpu]]
  percpu = true
  totalcpu = true
  collect_cpu_time = false
[[inputs.disk]]
  ignore_fs = ["tmpfs", "devtmpfs"]
[[inputs.diskio]]
[[inputs.kernel]]
[[inputs.mem]]
[[inputs.processes]]
[[inputs.swap]]
[[inputs.system]]
```

### Contact ###

* hello@orangesys.io
