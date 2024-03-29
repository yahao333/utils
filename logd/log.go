package logd

import (
	"errors"
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
	Ldate         // like  2006/01/02
	Ltime         // like 15:04:05
	Lmicroseconds // like 15:00:05.123123
	Llongfile     // like /a/b/c/d.go:23
	Lshortfile    // like d.go:23
	LUTC          // 时间utc输出
	Ldaily

	Lall = Ldebug | Linfo | Lwarn | Lerror | Lfatal
	// 2020/01/02 15:00:01.123412, /a/b/c/d.go:23
	LstdFlags = Ldate | Lmicroseconds | Lshortfile | Lall
)

var levelMaps = map[int]string{
	Ldebug: "DEBUG",
	Linfo:  "INFO",
	Lwarn:  "WARN",
	Lerror: "ERROR",
	Lfatal: "FATAL",
}

type Logger struct {
	mu    sync.Mutex
	obj   string      // 打印日志对象
	out   io.Writer   // 输出
	in    chan []byte // channel
	dir   string      // 输出目录
	flag  int         // 标志
	mails Emailer     // 告警邮件
}

type LogOption struct {
	Out        io.Writer // 输出writer
	LogDir     string    // 日志输出目录, 为空不输出到文件
	ChannelLen int       // channel
	Flag       int       // 标志位
	Mails      Emailer   // 告警邮件
}

func New(option LogOption) *Logger {
	wd, _ := os.Getwd()
	index := strings.LastIndex(wd, "/")
	logger := &Logger{
		obj:   wd[index+1:],
		out:   option.Out,
		in:    make(chan []byte, option.ChannelLen),
		dir:   option.LogDir,
		flag:  option.Flag,
		mails: option.Mails,
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
				l.obj, today.Format("2006-01-02")),
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
	if l.mails != nil && lvl >= Lwarn {
		go l.mails.SendMail(l.obj, buf)
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

func (l *Logger) formatHeader(buf *[]byte, lvl int, t time.Time, file string, line int) {
	if l.flag&LUTC != 0 {
		t = t.UTC()
	}
	if l.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		if (l.flag & Ldate) != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '/')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '/')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}
		if l.flag&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if l.flag&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
	}
	*buf = append(*buf, getColorLevel(levelMaps[lvl])...)
	*buf = append(*buf, ' ')
	if l.flag&(Lshortfile|Llongfile) != 0 {
		if l.flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, ": "...)
	}
}

func (l *Logger) WaitFlush() {
	for {
		if len(l.in) > 0 {
			time.Sleep(time.Nanosecond * 50)
		} else {
			break
		}
	}
}

// print
func (l *Logger) Printf(format string, v ...interface{}) {
	l.Output(Linfo, 2, fmt.Sprintf(format, v...))
}

func (l *Logger) Print(v ...interface{}) {
	l.Output(Linfo, 2, fmt.Sprintf(smartFormat(v...), v...))
}

// debug
func (l *Logger) Debugf(format string, v ...interface{}) {
	if Ldebug&l.flag != 0 {
		l.Output(Ldebug, 2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Debug(v string) {
	if Ldebug&l.flag != 0 {
		l.Output(Ldebug, 2, v)
	}
}

// info
func (l *Logger) Infof(format string, v ...interface{}) {
	if Linfo&l.flag != 0 {
		l.Output(Linfo, 2, fmt.Sprintf(format, v...))
	}
}
func (l *Logger) Info(v string) {
	if Linfo&l.flag != 0 {
		l.Output(Linfo, 2, v)
	}
}

// warn
func (l *Logger) Warnf(format string, v ...interface{}) {
	if Lwarn&l.flag != 0 {
		l.Output(Lwarn, 2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Warn(v string) {
	if Lwarn&l.flag != 0 {
		l.Output(Lwarn, 2, v)
	}
}

// error
func (l *Logger) Errorf(format string, v ...interface{}) {
	if Lerror&l.flag != 0 {
		l.Output(Lerror, 2, fmt.Sprintf(format, v...)+CallerStack())
	}
}

func (l *Logger) Error(v string) {
	if Lerror&l.flag != 0 {
		l.Output(Lerror, 2, v)
	}
}

// fatal
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.Output(Lfatal, 2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (l *Logger) Fatal(v string) {
	l.Output(Lfatal, 2, v)
	os.Exit(1)
}

func (l *Logger) Breakpoint() {
	l.Output(Ldebug, 3, fmt.Sprintln("breakpoint"))
}

func (l *Logger) SetLogDir(dir string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dir = dir
}

func (l *Logger) SetObj(obj string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.obj = obj
}

func (l *Logger) SetOutput(out io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = out
}

func (l *Logger) SetLevel(lvl int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var i uint
	for i = 0; i < uint(len(levelMaps)); i++ {
		if lvl&1 != 0 {
			break
		}
		lvl >>= 1
	}
	l.flag = (l.flag >> i) | (Lall >> i)
	l.flag <<= i
}

//----------------------------------- standard wrapper ---------------------------------
var Std = New(LogOption{Out: os.Stdout, ChannelLen: 1000, Flag: LstdFlags})

// RedirectLogFile 重新定义输出日志文件
func RedirectLogFile(logfile string, flags int) error {
	if logfile == "" {
		logfile = "app.log"
	}

	f, err := os.OpenFile(logfile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return errors.New("Could not open log file")
	}

	option := LogOption{
		Out:        f,
		Flag:       flags,
		ChannelLen: 1000,
	}
	Std = New(option)
	return nil
}

// 重新输出到文件
func Reset(logfile, token, secret string, en_ding bool) {
	if logfile != "" {
		Flags := Lwarn | Lerror | Lfatal | Ldate | Ltime | Lshortfile // | Ldaily  | Lasync
		f, _ := os.OpenFile(logfile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		option := LogOption{
			Out:        f,
			Flag:       Flags,
			ChannelLen: 1000,
		}
		/*
			if en_ding && token != "" && secret != "" {
				robot := NewRobot(token, secret)
				option.Ding = robot
			}
		*/

		Std = New(option)
	}
}

func Printf(format string, v ...interface{}) {
	Std.Output(Linfo, 2, fmt.Sprintf(format, v...))
}

func Print(v ...interface{}) {
	Std.Output(Linfo, 2, fmt.Sprintf(smartFormat(v...), v...))
}

func Debugf(format string, v ...interface{}) {
	if Ldebug&Std.flag != 0 {
		Std.Output(Ldebug, 2, fmt.Sprintf(format, v...))
	}
}
func Debug(v string) {
	if Ldebug&Std.flag != 0 {
		Std.Output(Ldebug, 2, v)
	}
}

func Infof(format string, v ...interface{}) {
	if Linfo&Std.flag != 0 {
		Std.Output(Linfo, 2, fmt.Sprintf(format, v...))
	}
}
func Info(v string) {
	if Linfo&Std.flag != 0 {
		Std.Output(Linfo, 2, v)
	}
}

func Warnf(format string, v ...interface{}) {
	if Lwarn&Std.flag != 0 {
		Std.Output(Lwarn, 2, fmt.Sprintf(format, v...))
	}
}

func Warn(v string) {
	if Lwarn&Std.flag != 0 {
		Std.Output(Lwarn, 2, v)
	}
}

func Errorf(format string, v ...interface{}) {
	if Lerror&Std.flag != 0 {
		Std.Output(Lerror, 2, fmt.Sprintf(format, v...)+"\n"+CallerStack())
	}
}

func Error(v string) {
	if Lerror&Std.flag != 0 {
		Std.Output(Lerror, 2, v)
	}
}

func Fatalf(format string, v ...interface{}) {
	Std.Output(Lfatal, 2, fmt.Sprintf(format, v...))
	Std.Output(Lfatal, 2, CallerStack())
	os.Exit(1)
}

func Fatal(v string) {
	Std.Output(Lfatal, 2, v)
	Std.Output(Lfatal, 2, CallerStack())
	os.Exit(1)
}

func Breakpoint() {
	Std.Breakpoint()
}
func SetLevel(lvl int) {
	Std.SetLevel(lvl)
}

func SetOutput(w io.Writer) {
	Std.SetOutput(w)
}

func SetObj(obj string) {
	Std.SetObj(obj)
}

//-----------------------------

// Cheap inteeger to fixed-width decimal ASCII. Give a nagative width to avoid zero-padding.
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

const (
	Gray = uint8(iota + 90)
	Red
	Green
	Yellow
	Blue
	Magenta
)

func getColorLevel(level string) string {
	level = strings.ToUpper(level)
	switch level {
	case "DEBUG":
		return fmt.Sprintf("\033[%dm[%5s]\033[0m", Green, level)
	case "INFO":
		return fmt.Sprintf("\033[%dm[%5s]\033[0m", Blue, level)
	case "WARN":
		return fmt.Sprintf("\033[%dm[%5s]\033[0m", Magenta, level)
	case "ERROR":
		return fmt.Sprintf("\033[%dm[%5s]\033[0m", Yellow, level)
	case "FATAL":
		return fmt.Sprintf("\033[%dm[%5s]\033[0m", Red, level)
	default:
		return level
	}
}

func CallerStack() string {
	var caller_str string
	for skip := 2; ; skip++ {
		// 获取调用者的信息
		pc, file, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		func_name := runtime.FuncForPC(pc).Name()
		caller_str += "Func : " + func_name + "\nFile:" + file + ":" + fmt.Sprint(line) + "\n"
	}
	return caller_str
}
func smartFormat(v ...interface{}) string {
	format := ""
	for i := 0; i < len(v); i++ {
		format += "%v"
	}
	format += "\n"
	return format
}
