package logd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	Ldebug = 1 << iota
	Linfo
	Lwarn
	Lerror
	Lfatal
	LAsync        // 异步输出日志
	Ldata         // like  2006/01/02
	Ltime         // like 15:04:05
	Lmicroseconds // like 15:00:05.123123
	Llongfile     // like /a/b/c/d.go:23
	Lshortfile    // like d.go:23
	LUTC          // 时间utc输出
	Ldaily

	Lall = Ldebug | Linfo | Lwarn | Lerror | Lfatal
	// 2020/01/02 15:00:01.123412, /a/b/c/d.go:23
	LstdFlags = Ldata | Lmicroseconds | Lshortfile | Lall
)

var levelMaps = map[int]string{
	Ldebug: "DEBUG",
	Linfo:  "INFO",
	Lwarn:  "WARN",
	Lerror: "ERROR",
	Lfatal: "FATAL",
}

type Logger struct {
	mu     sync.Mutex
	obj    string      // 打印日志对象
	out    io.Writer   // 输出
	in     chan []byte // channel
	dir    string      // 输出目录
	flag   int         // 标志
	emails []string    // 告警邮件
}

type LogOption struct {
	Out        io.Writer // 输出writer
	LogDir     string    // 日志输出目录, 为空不输出到文件
	ChannelLen int       // channel
	Flag       int       // 标志位
	Emails     []string  // 告警邮件
}

func New(option LogOption) *Logger {
	wd, _ := os.Getwd()
	index := strings.LastIndex(wd, "/")
	logger := &Logger{
		obj:    wd[index+1:],
		out:    option.Out,
		in:     make(chan []byte, option.ChannelLen),
		dir:    option.LogDir,
		flag:   option.Flag,
		emails: option.Emails,
	}
	if logger.flag|LAsync != 0 {
		go logger.receive()
	}
	return logger
}

func (l *Logger) receive() {
	today := time.Now()
	var file *os.File
	var err error
	for data := range l.in {
		if l.dir != "" && (file == nil || today.Day() != time.Now().Day()) {
			l.mu.Lock()
			today = time.Now()
			file, err = os.OpenFile(fmt.Sprintf("%s/%s_%s.log", l.dir,
				l.obj, today.Format("2020-01-01")),
				os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
			if err != nil {
				panic(err)
			}
			l.mu.Unlock()
			if (l.flag & Ldaily) != 0 {
				go l.rotate(today)
			}
		}
		if file != nil {
			file.Write(data)
		}
		if l.out != nil {
			l.out.Write(data)
		}
	}
}

// 压缩,依赖命令行gzip
func (l *Logger) rotate(t time.Time) {
	filepath.Walk(l.dir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if int(t.Sub(f.ModTime()).Hours()) > 24 {
			if strings.HasSuffix(f.Name(), ".log") {
				cmd := exec.Command("gzip", path)
				err = cmd.Run()
				if err != nil {
					return err
				}
			}
		}
		if int(t.Sub(f.ModTime()).Hours()) > 24*30 {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
}

// log format: date, time(hour:minute:second:microsecond), level, module, shortfile:line, <content>
func (l *Logger) Output(lvl int, calldepth int, content string) error {
	_, file, line, ok := runtime.Caller(calldepth)
	if !ok {
		return nil
	}

	var buf []byte
	l.formatHeader(&buf, lvl, time.Now(), file, line)
	buf = append(buf, content...)
	if len(l.emails) > 0 && lvl >= Lwarn {
		go sendMail(l.obj, buf, l.emails)
	}
	if l.flag&LAsync != 0 {
		l.in <- buf
	} else {
		l.mu.Lock()
		defer l.mu.Unlock()

		l.out.Write(buf)
	}
	return nil
}
