package captcha

import (
	"fmt"
	"testing"
)

func TestA(t *testing.T){
	param := configJsonBody{Id: "1", CaptchaType: "string", CaptchaValue: "1234", VerifyValue: ""}
	val := CreateCaptcha()
	param.VerifyValue = val
	store.Set(param.Id, val)

	if store.Verify(param.Id, val, true) {
		fmt.Println("[Pass]")
	}else{
		fmt.Println("[Fail]")
	}
}