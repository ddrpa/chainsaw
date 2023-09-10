package main

import (
	"bufio"
	"fmt"
	"github.com/akamensky/argparse"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	Version = "1.0.0"
)

var waitGroup sync.WaitGroup
var outputDir string
var dryRun bool

func main() {
	parser := argparse.NewParser("chainsaw", "Cut large log file into small pieces, version "+Version)
	notBeforeDate := parser.Int("", "not-before", &argparse.Options{Required: false, Help: "Ignore logs before, format 20230908"})
	notAfterDate := parser.Int("", "not-after", &argparse.Options{Required: false, Help: "Ignore logs after, format 20230908"})
	// 日志中文本的数量会影响最终文件的大小
	// 作为参考，20000 行（约 2.5 MB）以上的文件可能无法正确地高亮字符
	// 50000 行（约 6 MB）以上的文件可能无法正确地显示行号
	// 注意这不代表单个日志文件一定不会超过这个行数，因为 chainsaw 会尽量保证同一条日志不会被分割到两个文件中（例如堆栈信息）
	chunkSize := parser.Int("", "chunk-size", &argparse.Options{Required: false, Default: 50000, Help: "Max lines per file"})
	logfile := parser.String("f", "file", &argparse.Options{Required: true, Help: "Original log file"})
	argsOutputDir := parser.String("o", "output", &argparse.Options{Required: false, Default: "cut/", Help: "Output directory"})
	argsDryRun := parser.Flag("", "dry-run", &argparse.Options{Required: false, Help: "No file will be written"})

	err := parser.Parse(os.Args)
	if err != nil {
		log.Fatal(parser.Usage(err))
	}

	outputDir = *argsOutputDir
	// ensure output dir exists
	_ = os.Mkdir(outputDir, 0777)

	dryRun = *argsDryRun
	if dryRun {
		fmt.Println("Running in dry-run mode, no file will be written")
	}

	var notBeforeFilter func(int) bool
	if *notBeforeDate > 0 {
		notBeforeFilter = func(messageTimestampAsInt int) bool {
			return messageTimestampAsInt >= *notBeforeDate
		}
	} else {
		notBeforeFilter = func(messageTimestampAsInt int) bool {
			return true
		}
	}
	var notAfterFilter func(int) bool
	if *notAfterDate > 0 {
		notAfterFilter = func(messageTimestampAsInt int) bool {
			return messageTimestampAsInt <= *notAfterDate
		}
	} else {
		notAfterFilter = func(messageTimestampAsInt int) bool {
			return true
		}
	}

	countTotal := 0
	countDropped := 0
	countPassed := 0
	countProcessed := 0

	haveStarted := false
	passLogMessageByDate := false
	notAfterFilterSkipped := false

	chunkCounter := 0

	// something like 2023-06-16 16:26:45.495  INFO ...
	//datetimePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}.[0-9]{3}`)
	datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
	var cursorTimestamp string
	logBuffer := make([]string, 0)

	file, _ := os.Open(*logfile)
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		countTotal += 1
		line := scanner.Text()
		match := datePattern.FindStringSubmatch(line)
		isNewLogLine := len(match) > 0

		if haveStarted == false {
			// 还未匹配到过时间戳时读取的行不知道该归类到哪个文件，因此直接丢弃
			if isNewLogLine {
				haveStarted = true
				cursorTimestamp = match[0]
			} else {
				countDropped += 1
				continue
			}
		}

		if passLogMessageByDate {
			// 当前处于跳过模式，判断是否需要解除跳过
			if isNewLogLine {
				// 转换为数字，方便比较
				logTimestampInt, _ := strconv.Atoi(strings.ReplaceAll(match[0], "-", ""))
				if notBeforeFilter(logTimestampInt) && notAfterFilter(logTimestampInt) {
					// 解除跳过状态
					passLogMessageByDate = false
				} else {
					countPassed += 1
					continue
				}
			} else {
				countPassed += 1
				continue
			}
		} else {
			// 判断是否需要进入跳过模式
			if isNewLogLine {
				logTimestampInt, _ := strconv.Atoi(strings.ReplaceAll(match[0], "-", ""))
				if notBeforeFilter(logTimestampInt) == false {
					passLogMessageByDate = true
					countPassed += 1
					continue
				} else if notAfterFilter(logTimestampInt) == false {
					// 不统计之后的日志
					notAfterFilterSkipped = true
					break
				}
			}
		}

		countProcessed += 1
		if !isNewLogLine {
			// 不是新日志行，添加到现有缓冲区
			logBuffer = append(logBuffer, line)
			continue
		}
		messageTimestamp := match[0]
		if cursorTimestamp == messageTimestamp {
			// 匹配时间戳游标
			if len(logBuffer) > *chunkSize {
				// 判断缓冲区大小决定是否需要分块
				waitGroup.Add(1)
				go saveLog(logBuffer, cursorTimestamp, true, chunkCounter)
				chunkCounter += 1
				logBuffer = make([]string, 0)
			}
		} else {
			// 不匹配时间戳游标
			waitGroup.Add(1)
			go saveLog(logBuffer, cursorTimestamp, chunkCounter != 0, chunkCounter)
			chunkCounter = 0
			logBuffer = make([]string, 0)
			cursorTimestamp = messageTimestamp
		}
		logBuffer = append(logBuffer, line)
	}
	// 结束循环，保存最后一个缓冲区
	if haveStarted {
		waitGroup.Add(1)
		go saveLog(logBuffer, cursorTimestamp, chunkCounter != 0, chunkCounter)
	}
	waitGroup.Wait()
	fmt.Println("")
	if notAfterFilterSkipped {
		fmt.Println("Not after filter used, lines after specified date skipped")
		fmt.Printf("%d lines dropped\n", countDropped)
		fmt.Printf("%d lines saved\n", countProcessed)
	} else {
		fmt.Printf("%d lines dropped\n", countDropped)
		fmt.Printf("%d lines saved\n", countProcessed)
		fmt.Printf("%d lines passed\n", countPassed)
		fmt.Printf("%d lines in given file\n", countTotal)
	}
}

func saveLog(logBuffer []string, timestamp string, chunked bool, chunkNumber int) {
	if len(logBuffer) <= 0 {
		waitGroup.Done()
		return
	}
	var filename string
	if chunked {
		filename = path.Join(outputDir, fmt.Sprintf("%s.%d.log", timestamp, chunkNumber))
	} else {
		filename = path.Join(outputDir, fmt.Sprintf("%s.log", timestamp))
	}
	if !dryRun {
		file, _ := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0666)
		defer func(file *os.File) {
			_ = file.Close()
		}(file)
		for _, line := range logBuffer {
			_, _ = file.WriteString(line + "\n")
		}
	}
	fmt.Printf("%s saved (%d lines)\n", filename, len(logBuffer))
	waitGroup.Done()
}
