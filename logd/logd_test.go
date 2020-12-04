package logd

import "testing"

func TestAll(t *testing.T) {
	Printf("Print: foo\n")
	Print("Print: foo")

	SetLevel(Ldebug)

	Debugf("Debug: foo\n")
	Debug("Debug: foo")

	Infof("Info: foo\n")
	Info("Info: foo")

	Errorf("Error: foo")
	Error("Error: foo")

	SetLevel(Lerror)

	Debugf("Debug: foo\n")
	Debug("Debug: foo")

	Infof("Info: foo\n")
	Info("Info: foo")

	Errorf("Error: foo")
	Error("Error: foo")
}
