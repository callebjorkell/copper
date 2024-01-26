package copper

import (
	"fmt"
)

var _ error = SentinelError{}

type SentinelError struct {
	msg string
}

func (s SentinelError) Error() string {
	return s.msg
}

var (
	ErrNotChecked      = SentinelError{"not checked"}
	ErrNotPartOfSpec   = SentinelError{"not part of spec"}
	ErrResponseInvalid = SentinelError{"response invalid"}
	ErrRequestInvalid  = SentinelError{"request invalid"}
)

func joinError(sentinel SentinelError, err error) *VerificationError {
	return &VerificationError{
		err:      err,
		sentinel: sentinel,
	}
}

type VerificationError struct {
	err      error
	sentinel SentinelError
}

func (v *VerificationError) Sentinel() error {
	return v.sentinel
}

func (v *VerificationError) Error() string {
	return fmt.Sprintf("%v: %v", v.sentinel.Error(), v.err.Error())
}

func (v *VerificationError) Unwrap() []error {
	return []error{v.err, v.sentinel}
}
