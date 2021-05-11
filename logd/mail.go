package logd

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

type Emailer interface {
	SendMail(fromname string, msg []byte) error
}

type Smtp struct {
	From    string   // 发件箱: xx@163.com
	Key     string   // 发件密钥: sdkfjakdfj
	Host    string   // 主机地址： smtp.example.com
	Port    string   // 主机端口: 465
	To      []string // 发送给: xxx@163.com
	Subject string   // 邮件标题: 告警[logd]
}

func (s *Smtp) SendMail(fromname string, msg []byte) error {
	conn, err := tls.Dial("tcp", s.Host+":"+s.Port, nil)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return err
	}
	// 获取授权
	auth := smtp.PlainAuth("", s.From, s.Key, s.Host)
	if err = client.Auth(auth); err != nil {
		return err
	}

	if err = client.Mail(s.From); err != nil {
		return err
	}

	str := fmt.Sprint(
		"To:", strings.Join(s.To, ","),
		"\r\nFrom:", fmt.Sprintf("%s<%s>", fromname, s.From),
		"\r\nSubject:", s.Subject,
		"\r\n", "Content-Type:text/plain;charset=UTF-8",
		"\r\n\r\n",
	)
	data := make([]byte, len(str)+len(msg))
	copy(data, []byte(str))
	copy(data[len(str):], msg)

	// RCPT
	for _, d := range s.To{
		if err := client.Rcpt(d); err != nil{
			return  err
		}
	}

	// 获取WriteCloser
	wc, err := client.Data()
	if err != nil{
		return err
	}

	// 写入数据
	_, err = wc.Write(data)
	if err != nil{
		return err
	}
	wc.Close()

	return client.Quit()
}
