package captcha

import(
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"time"
)

var store = DefaultMemStore

type configJsonBody struct {
	Id          string
	CaptchaType string
	VerifyValue string
	CaptchaValue string
}

func CreateCaptcha() string {
	return fmt.Sprintf("%04v", rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(1000000))
}
func DriverDigitFunc()(id, bs string, err error){
	e := configJsonBody{}
	e.Id = uuid.New().String()
	e.CaptchaType = "string"
	val := CreateCaptcha()
	e.CaptchaValue = val
	e.VerifyValue = val
	store.Set(e.Id, val)

	return e.Id, e.CaptchaValue, nil
}
