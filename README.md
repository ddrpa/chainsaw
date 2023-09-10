# Chainsaw - Cut large log file into small pieces

chainsaw 可以帮助你把数个 GB 大小的单一日志文件按日期切分，以便充分释放一些文本搜索工具的多线程能力。

作者希望你永远碰不上使用它的场景，但如果你真的倒霉了，那么它应该是有点用的。

```sh
$ chainsaw --help

usage: chainsaw [-h|--help] [--not-before <integer>] [--not-after <integer>]
                [-c|--chunk-size <integer>] -f|--file "<value>" [-o|--output
                "<value>"] [--dry-run]

                Cut large log file into small pieces, version 0.1.0

Arguments:

  -h  --help        Print help information
      --not-before  Ignore logs before, format 20230908
      --not-after   Ignore logs after, format 20230908
      --chunk-size  Max lines per file. Default: 50000
  -f  --file        Original log file
  -o  --output      Output directory. Default: cut/
      --dry-run     No file will be written
```

日志中文本的数量会影响最终文件的大小。作为参考，在某些编辑器中，20000 行（约 2.5 MB）以上的文件可能无法正确地高亮字符，50000 行（约 6 MB）以上的文件可能无法正确地显示行号。
chainsaw 默认设置 50000 行的软上限，即每个切分文件最多 50000 行，但实际上会尽量保证同一条日志不会被分割到两个文件中（例如堆栈信息），因此一般会超过这个值。

```sh
$ chainsaw --dry-run -f analyze.log

Running in dry-run mode, no file will be written
cut/2023-06-16.log saved (660 lines)
cut/2023-06-19.log saved (166 lines)
...
cut/2023-08-04.22.log saved(820 lines)

20 lines dropped
14042641 lines saved
0 lines passed
14042661 lines in given file
```

从今天起，检查 `/etc/logrotate.d/` 目录下的配置，看看 `logrotated` 有没有运行。
看看代码的日志部分有没有配置 `RollingFileAppender` 或者接入什么 Cloud-Based Log Service。
特别地，见到谁用 `nohup *** > app.log 2>&1 &` 就揍他。
