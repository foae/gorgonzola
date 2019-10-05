package dns

import "fmt"

type Err struct {
	fatal bool
	msg   string
}

func (e *Err) Error() string {
	return e.msg
}

func (e *Err) IsFatal() bool {
	return e.fatal
}

func errFatal(msg string, args ...interface{}) error {
	return &Err{
		fatal: true,
		msg:   fmt.Sprintf(msg, args...),
	}
}

func errRec(msg string, args ...interface{}) error {
	return &Err{
		fatal: false,
		msg:   fmt.Sprintf(msg, args...),
	}
}
