# Diskspace2slack

Report critical disk space values directly to Slack!

Usage

```
./diskspace2slack -h
Usage of diskspace2slack:
  -disk string
        Disk names as Strings, separated by space. (default "/ /tmp")
  -target string
        Target Person or Channel on Slack. (default "#target_slack_channel")
  -threshold string
        Integers representing the maximum percentage of free space before alerting, seperated by spaces. (default "10 10")
```

Example

```
set SLACK_SECRET_KEY="" \
./diskspace2slack -disk "/" -threshold "90" -target "@user_name"
```