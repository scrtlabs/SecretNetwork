package types

import (
	"fmt"
	"reflect"
)

// StdError captures all errors returned from the Rust code as StdError.
// Exactly one of the fields should be set.
type StdError struct {
	GenericErr    *GenericErr    `json:"generic_err,omitempty"`
	InvalidBase64 *InvalidBase64 `json:"invalid_base64,omitempty"`
	InvalidUtf8   *InvalidUtf8   `json:"invalid_utf8,omitempty"`
	NotFound      *NotFound      `json:"not_found,omitempty"`
	ParseErr      *ParseErr      `json:"parse_err,omitempty"`
	SerializeErr  *SerializeErr  `json:"serialize_err,omitempty"`
	Unauthorized  *Unauthorized  `json:"unauthorized,omitempty"`
	Underflow     *Underflow     `json:"underflow,omitempty"`
}

var (
	_ error = StdError{}
	_ error = GenericErr{}
	_ error = InvalidBase64{}
	_ error = InvalidUtf8{}
	_ error = NotFound{}
	_ error = ParseErr{}
	_ error = SerializeErr{}
	_ error = Unauthorized{}
	_ error = Underflow{}
)

func (a StdError) Error() string {
	switch {
	case a.GenericErr != nil:
		return a.GenericErr.Error()
	case a.InvalidBase64 != nil:
		return a.InvalidBase64.Error()
	case a.InvalidUtf8 != nil:
		return a.InvalidUtf8.Error()
	case a.NotFound != nil:
		return a.NotFound.Error()
	case a.ParseErr != nil:
		return a.ParseErr.Error()
	case a.SerializeErr != nil:
		return a.SerializeErr.Error()
	case a.Unauthorized != nil:
		return a.Unauthorized.Error()
	case a.Underflow != nil:
		return a.Underflow.Error()
	default:
		panic("unknown error variant")
	}
}

type GenericErr struct {
	Msg string `json:"msg,omitempty"`
}

func (e GenericErr) Error() string {
	return fmt.Sprintf("encrypted: %s", e.Msg)
}

type InvalidBase64 struct {
	Msg string `json:"msg,omitempty"`
}

func (e InvalidBase64) Error() string {
	return fmt.Sprintf("invalid base64: %s", e.Msg)
}

type InvalidUtf8 struct {
	Msg string `json:"msg,omitempty"`
}

func (e InvalidUtf8) Error() string {
	return fmt.Sprintf("invalid_utf8: %s", e.Msg)
}

type NotFound struct {
	Kind string `json:"kind,omitempty"`
}

func (e NotFound) Error() string {
	return fmt.Sprintf("not found: %s", e.Kind)
}

type ParseErr struct {
	Target string `json:"target,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

func (e ParseErr) Error() string {
	return fmt.Sprintf("parsing %s: %s", e.Target, e.Msg)
}

type SerializeErr struct {
	Source string `json:"source,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

func (e SerializeErr) Error() string {
	return fmt.Sprintf("serializing %s: %s", e.Source, e.Msg)
}

type Unauthorized struct{}

func (e Unauthorized) Error() string {
	return "unauthorized"
}

type Underflow struct {
	Minuend    string `json:"minuend,omitempty"`
	Subtrahend string `json:"subtrahend,omitempty"`
}

func (e Underflow) Error() string {
	return fmt.Sprintf("underflow: %s - %s", e.Minuend, e.Subtrahend)
}

// ToStdError will convert the given error to an StdError.
// This is important to returning any Go error back to Rust.
//
// If it is already StdError, return self.
// If it is an error, which could be a sub-field of StdError, embed it.
// If it is anything else, convert it to a GenericErr.
func ToStdError(err error) *StdError {
	if isNil(err) {
		return nil
	}
	switch t := err.(type) {
	case StdError:
		return &t
	case *StdError:
		return t
	case GenericErr:
		return &StdError{GenericErr: &t}
	case *GenericErr:
		return &StdError{GenericErr: t}
	case InvalidBase64:
		return &StdError{InvalidBase64: &t}
	case *InvalidBase64:
		return &StdError{InvalidBase64: t}
	case InvalidUtf8:
		return &StdError{InvalidUtf8: &t}
	case *InvalidUtf8:
		return &StdError{InvalidUtf8: t}
	case NotFound:
		return &StdError{NotFound: &t}
	case *NotFound:
		return &StdError{NotFound: t}
	case ParseErr:
		return &StdError{ParseErr: &t}
	case *ParseErr:
		return &StdError{ParseErr: t}
	case SerializeErr:
		return &StdError{SerializeErr: &t}
	case *SerializeErr:
		return &StdError{SerializeErr: t}
	case Unauthorized:
		return &StdError{Unauthorized: &t}
	case *Unauthorized:
		return &StdError{Unauthorized: t}
	case Underflow:
		return &StdError{Underflow: &t}
	case *Underflow:
		return &StdError{Underflow: t}
	default:
		g := GenericErr{Msg: err.Error()}
		return &StdError{GenericErr: &g}
	}
}

// check if an interface is nil (even if it has type info)
func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	if reflect.TypeOf(i).Kind() == reflect.Ptr {
		// IsNil panics if you try it on a struct (not a pointer)
		return reflect.ValueOf(i).IsNil()
	}
	// if we aren't a pointer, can't be nil, can we?
	return false
}
