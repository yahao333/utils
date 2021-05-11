package logd

import "testing"

func TestSmtp_SendMail(t *testing.T) {
	s := &Smtp{
		From:    "ubt2012@163.com",
		Key:     "xxxxxxx",
		Host:    "smtp.163.com",
		Port:    "465",
		To:      []string{"fexeak@163.com"},
		Subject: "test email from log",
	}

	err := s.SendMail("test", []byte("hello world"))
	if err != nil {
		t.Error(err)
	}
}
