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
[[outputs.orangesys]]
Urls = "https://demo.i.orangesys.io"
token = "token"
prefix = "telegraf."
debug = false

```

### Contact ###

* hello@orangesys.io
