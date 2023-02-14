package mailer

import (
	"reflect"
	"unsafe"
)

// string2bytes converts the given string to a byte slice without memory allocation.
//
// Note it may break if string and/or slice header will change in future go versions.
func string2bytes(s string) (b []byte) {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := *(*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len

	return b
}

// byteSlice2String converts a byte slice to a string in a performant way.
func byteSlice2String(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}
