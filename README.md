# psm_go_client

A go client for psm. Support mutiple region and mutiple type of monitor.
Use go's goroutine to monitor large and dense monitor jobs.

PHP Server Monitor http://www.phpservermonitor.org

step 1, install psm
===================

install docs:

https://github.com/nutsteam/phpservermon/tree/nutsmobi#mutiple-region


step 2, add cron
===================

php version:
```shell
php cron/status.cron.php
```

go version:
```shell
psm_go_client -url=http://your psm server url/ -token=YOUTOKENHERE -region=YOUREGIN
```


# psm_go_client

Go版本的psm客户端，支持多节点、多种类型的监控任务。
通过Go的并发执行监控，可以在数秒内完成大量监控任务，适合任务数量多，监控周期特别密集的需求。

PHP Server Monitor http://www.phpservermonitor.org

step 1, 安装 psm
===================

先安装psm，再安装 psm 多节点的版本:

https://github.com/nutsteam/phpservermon/tree/nutsmobi#mutiple-region


step 2, 替换计划任务
===================

php 版本的任务:
```shell
php cron/status.cron.php
```

go 版本的任务:
```shell
psm_go_client -url=http://your psm server url/ -token=YOUTOKENHERE -region=YOUREGIN
```
