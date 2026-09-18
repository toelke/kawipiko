module github.com/volution/kawipiko

go 1.21

require (
	github.com/andybalholm/brotli v1.0.4
	github.com/colinmarc/cdb v0.0.0-20190223170904-60f317823f70
	github.com/foobaz/go-zopfli v0.0.0-20140122214029-7432051485e2
	github.com/valyala/fasthttp v1.40.0
	github.com/valyala/tcplisten v1.0.0
	github.com/zeebo/blake3 v0.2.3
	go.etcd.io/bbolt v1.3.6
	golang.org/x/sys v0.0.0-20220907062415-87db552b00fd
)

require (
	github.com/Pallinder/go-randomdata v1.2.0 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/klauspost/cpuid/v2 v2.1.1 // indirect
	github.com/stretchr/testify v1.8.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/zeebo/assert v1.3.1 // indirect
)

replace github.com/colinmarc/cdb => github.com/cipriancraciun/go-cdb-lib v0.0.0-20190809203657-d959ce9cc674
