package tmpl

import (
	"encoding/base64"
	htmpl "html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"text/template"
	"time"
)

var TplFuncMap = make(template.FuncMap)

func DateFormat(t time.Time, layout string) string {
	return t.Format(layout)
}
func Str2html(raw string) htmpl.HTML {
	return htmpl.HTML(raw)
}
func Join(a []string, sep string) string {
	return strings.Join(a, sep)
}
func IsNotZero(t time.Time) bool {
	return !t.IsZero()
}

// cache avatar image
var avatar string

func GetAvatar(domain string) string {
	if avatar == "" {
		resp, err := http.Get("http://" + domain + "/static/img/avatar.png")
		if err != nil {
			log.Println(err)
			return ""
		}
		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			log.Println(err)
			return ""
		}

		avatar = "data:" + resp.Header.Get("content-type") + ";base64," + base64.StdEncoding.EncodeToString(data)
	}

	return avatar
}

func init() {
	TplFuncMap["dateFormat"] = DateFormat
	TplFuncMap["str2html"] = Str2html
	TplFuncMap["join"] = Join
	TplFuncMap["isnotzero"] = IsNotZero
	TplFuncMap["getavatar"] = GetAvatar
}
