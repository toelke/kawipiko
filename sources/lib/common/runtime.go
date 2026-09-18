

package common


import "unsafe"




//go:nosplit
func NoEscape (p unsafe.Pointer) (unsafe.Pointer) {
	x := uintptr (p)
	return unsafe.Pointer (x ^ 0)
}




func NoEscapeBytes (_input *[]byte) (*[]byte) {
	return (*[]byte) (NoEscape (unsafe.Pointer (_input)))
}


func NoEscapeString (_input *string) (*string) {
	return (*string) (NoEscape (unsafe.Pointer (_input)))
}




func BytesToString (_input []byte) (string) {
	
	// NOTE:  Since Go 1.20 this is the supported way to alias a `[]byte`'s
	//        backing array as a `string`, without any copying.
	//        (It replaces the previous `reflect.SliceHeader` / `reflect.StringHeader`
	//        manipulation, which was not GC-safe.)
	
	if len (_input) == 0 {
		return ""
	}
	
	return unsafe.String (unsafe.SliceData (_input), len (_input))
}


func StringToBytes (_input string) ([]byte) {
	
	// NOTE:  Since Go 1.20 this is the supported way to alias a `string`'s
	//        backing array as a `[]byte`, without any copying.
	//        (It replaces the previous `reflect.SliceHeader` / `reflect.StringHeader`
	//        manipulation, which was not GC-safe.)
	
	// WARNING:  The resulting `[]byte` aliases immutable `string` memory,
	//           therefore it must never be written to!
	
	if len (_input) == 0 {
		return nil
	}
	
	return unsafe.Slice (unsafe.StringData (_input), len (_input))
}




// NOTE:  https://github.com/aristanetworks/goarista/blob/master/monotime/nanotime.go

//go:noescape
//go:linkname runtime_nanotime runtime.nanotime
func runtime_nanotime () (int64)

func RuntimeNanoseconds () (uint64) {
	return uint64 (runtime_nanotime ())
}

func RuntimeMicroseconds () (uint64) {
	return uint64 (runtime_nanotime ()) / 1000
}

func RuntimeMilliseconds () (uint64) {
	return uint64 (runtime_nanotime ()) / 1000 / 1000
}

func RuntimeSeconds () (uint64) {
	return uint64 (runtime_nanotime ()) / 1000 / 1000 / 1000
}

func RuntimeSecondsFloat () (float64) {
	return float64 (runtime_nanotime ()) / 1000 / 1000 / 1000
}

func RuntimeHoursFloat () (float64) {
	return float64 (RuntimeSeconds ()) / 3600
}

