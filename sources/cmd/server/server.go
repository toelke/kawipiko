

package server


import "bytes"
import "context"
import "crypto/tls"
import "flag"
import "fmt"
import "io"
import "io/ioutil"
import "log"
import "net"
import "net/http"
import "os"
import "os/signal"
import "runtime"
import "runtime/debug"
import "runtime/pprof"
import "strconv"
import "strings"
import "sync"
import "syscall"
import "time"
import "unsafe"

import "github.com/colinmarc/cdb"

import "github.com/valyala/fasthttp"
import "github.com/valyala/fasthttp/reuseport"

import "github.com/lucas-clemente/quic-go"
import "github.com/lucas-clemente/quic-go/http3"

import . "github.com/volution/kawipiko/lib/common"
import . "github.com/volution/kawipiko/lib/server"

import _ "embed"




type server struct {
	httpServer *fasthttp.Server
	httpsServer *fasthttp.Server
	https2Server *http.Server
	quicServer *http3.Server
	cdbReader *cdb.CDB
	cachedFileFingerprints map[string][]byte
	cachedDataMeta map[string][]byte
	cachedDataContent map[string][]byte
	securityHeadersEnabled bool
	securityHeadersTls bool
	http1Disabled bool
	http2Disabled bool
	http3AltSvc string
	debug bool
	quiet bool
	dummy bool
	delay time.Duration
}




func (_server *server) Serve (_context *fasthttp.RequestCtx) () {
	
	
	if _server.dummy {
		_server.ServeDummy (_context)
		return
	}
	
	
	_request := (*fasthttp.Request) (NoEscape (unsafe.Pointer (&_context.Request)))
	_requestHeaders := (*fasthttp.RequestHeader) (NoEscape (unsafe.Pointer (&_request.Header)))
	_response := (*fasthttp.Response) (NoEscape (unsafe.Pointer (&_context.Response)))
	_responseHeaders := (*fasthttp.ResponseHeader) (NoEscape (unsafe.Pointer (&_response.Header)))
	
	_requestMethod := _requestHeaders.Method ()
	_requestUri := _requestHeaders.RequestURI ()
	_requestUriString_0 := BytesToString (_requestUri)
	_requestUriString := NoEscapeString (&_requestUriString_0)
	
	
	
	
	_pathBuffer := [512]byte {}
	_keyBufferLarge := [512]byte {}
	_keyBufferSmall := [128]byte {}
	
	_path := _pathBuffer[:0]
	_path = append (_path, _requestUri ...)
	if _pathLimit := bytes.IndexByte (_path, '?'); _pathLimit > 0 {
		_path = _path[: _pathLimit]
	}
	
	// FIXME:  Decode path according to `decodeArgAppendNoPlus`!
	
	_pathLen := len (_path)
	
	if ! bytes.Equal (StringToBytes (http.MethodGet), _requestMethod) {
		if !_server.quiet {
			log.Printf ("[ww] [bce7a75b]  [http-x..]  invalid method `%s` for `%s`!\n", BytesToString (_requestHeaders.Method ()), *_requestUriString)
		}
		_server.ServeError (_context, http.StatusMethodNotAllowed, nil, true)
		return
	}
	if (_pathLen == 0) || (_path[0] != '/') {
		if !_server.quiet {
			log.Printf ("[ww] [fa6b1923]  [http-x..]  invalid path `%s`!\n", *_requestUriString)
		}
		_server.ServeError (_context, http.StatusBadRequest, nil, true)
		return
	}
	
	_pathIsRoot := _pathLen == 1
	_pathHasSlash := !_pathIsRoot && (_path[_pathLen - 1] == '/')
	
	if bytes.HasPrefix (_path, StringToBytes ("/__/")) {
		if bytes.Equal (_path, StringToBytes ("/__/heartbeat")) || bytes.HasPrefix (_path, StringToBytes ("/__/heartbeat/")) {
			_server.ServeStatic (_context, http.StatusOK, HeartbeatDataOk, HeartbeatContentType, HeartbeatContentEncoding, false)
			return
		} else if bytes.Equal (_path, StringToBytes ("/__/about")) || bytes.Equal (_path, StringToBytes ("/__/about/")) {
			_server.ServeStatic (_context, http.StatusOK, AboutBannerData, AboutBannerContentType, AboutBannerContentEncoding, true)
			return
		} else if bytes.HasPrefix (_path, StringToBytes ("/__/banners/errors/")) {
			_code := _path[len ("/__/banners/errors/") :]
			if _code, _error := strconv.Atoi (BytesToString (*NoEscapeBytes (&_code))); _error == nil {
				_banner, _bannerFound := ErrorBannersData[uint (_code)]
				if (_code > 0) && _bannerFound {
					_server.ServeStatic (_context, http.StatusOK, _banner, ErrorBannerContentType, ErrorBannerContentEncoding, true)
					return
				}
			}
			_server.ServeError (_context, http.StatusBadRequest, nil, true)
			return
		} else {
			_server.ServeError (_context, http.StatusBadRequest, nil, true)
			return
		}
	}
	
	var _fingerprints []byte
	
	var _namespaceAndPathSuffixes = [][2]string {
			{NamespaceFilesContent, ""},
			{NamespaceFilesContent, "/"},
			{NamespaceFoldersContent, ""},
		}
	
	if _fingerprints == nil {
		_loop_1 : for _namespaceAndPathSuffixIndex := range _namespaceAndPathSuffixes {
			_namespaceAndPathSuffix := _namespaceAndPathSuffixes[_namespaceAndPathSuffixIndex]
			_namespace := _namespaceAndPathSuffix[0]
			_pathSuffix := _namespaceAndPathSuffix[1]
			
			switch {
				case !_pathIsRoot && !_pathHasSlash :
					// NOP
				case _pathSuffix == "/" :
					continue _loop_1
				case _pathSuffix == "" :
					// NOP
				case _pathSuffix[0] == '/' :
					_pathSuffix = _pathSuffix[1:]
			}
			_pathSuffixHasSlash := (len (_pathSuffix) != 0) && (_pathSuffix[0] == '/')
			
			if _server.cachedFileFingerprints != nil {
				_key := _keyBufferLarge[:0]
				_key = append (_key, _path ...)
				_key = append (_key, _pathSuffix ...)
				_fingerprints, _ = _server.cachedFileFingerprints[BytesToString (*NoEscapeBytes (&_key))]
			} else {
				_key := _keyBufferLarge[:0]
				_key = append (_key, _namespace ...)
				_key = append (_key, ':')
				_key = append (_key, _path ...)
				_key = append (_key, _pathSuffix ...)
				if _value, _error := _server.cdbReader.GetWithCdbHash (_key); _error == nil {
					_fingerprints = _value
				} else {
					_server.ServeError (_context, http.StatusInternalServerError, _error, false)
					return
				}
			}
			
			if _fingerprints != nil {
				if ((_namespace == NamespaceFoldersContent) || _pathSuffixHasSlash) && (!_pathIsRoot && !_pathHasSlash) {
					_path = append (_path, '/')
					_server.ServeRedirect (_context, http.StatusTemporaryRedirect, _path, true)
					return
				}
				break _loop_1
			}
		}
	}
	
	if _fingerprints == nil {
		if bytes.Equal (StringToBytes ("/favicon.ico"), _path) {
			_server.ServeStatic (_context, http.StatusOK, FaviconData, FaviconContentType, FaviconContentEncoding, true)
			return
		}
	}
	
	if _fingerprints == nil {
		_loop_2 : for
				_pathLimit := bytes.LastIndexByte (_path, '/');
				_pathLimit >= 0;
				_pathLimit = bytes.LastIndexByte (_path[: _pathLimit], '/') {
			
			if _server.cachedFileFingerprints != nil {
				_key := _keyBufferLarge[:0]
				_key = append (_key, _path[: _pathLimit] ...)
				_key = append (_key, "/*" ...)
				_fingerprints, _ = _server.cachedFileFingerprints[BytesToString (*NoEscapeBytes (&_key))]
			} else {
				_key := _keyBufferLarge[:0]
				_key = append (_key, NamespaceFilesContent ...)
				_key = append (_key, ':')
				_key = append (_key, _path[: _pathLimit] ...)
				_key = append (_key, "/*" ...)
				if _value, _error := _server.cdbReader.GetWithCdbHash (_key); _error == nil {
					_fingerprints = _value
				} else {
					_server.ServeError (_context, http.StatusInternalServerError, _error, false)
					return
				}
			}
			
			if _fingerprints != nil {
				break _loop_2
			}
		}
	}
	
	if _fingerprints == nil {
		if !_server.quiet {
			log.Printf ("[ww] [7416f61d]  [http-x..]  not found `%s`!\n", *_requestUriString)
		}
		_server.ServeError (_context, http.StatusNotFound, nil, true)
		return
	}
	
	_fingerprintsSplit := bytes.IndexByte (_fingerprints, ':')
	if _fingerprintsSplit < 0 {
		if !_server.quiet {
			log.Printf ("[ee] [7ee6c981]  [cdb.....]  invalid data fingerprints for `%s`!\n", *_requestUriString)
		}
		_server.ServeError (_context, http.StatusInternalServerError, nil, false)
		return
	}
	_fingerprintMeta := _fingerprints[:_fingerprintsSplit]
	_fingerprintContent := _fingerprints[_fingerprintsSplit + 1:]
	
	var _data []byte
	if _server.cachedDataContent != nil {
		_data, _ = _server.cachedDataContent[BytesToString (_fingerprintContent)]
	} else {
		_key := _keyBufferSmall[:0]
		_key = append (_key, NamespaceDataContent ...)
		_key = append (_key, ':')
		_key = append (_key, _fingerprintContent ...)
		if _value, _error := _server.cdbReader.GetWithCdbHash (_key); _error == nil {
			_data = _value
		} else {
			_server.ServeError (_context, http.StatusInternalServerError, _error, false)
			return
		}
	}
	if _data == nil {
		if !_server.quiet {
			log.Printf ("[ee] [0165c193]  [cdb.....]  missing data content for `%s`!\n", *_requestUriString)
		}
		_server.ServeError (_context, http.StatusInternalServerError, nil, false)
		return
	}
	
	var _dataMetaRaw []byte
	if _server.cachedDataMeta != nil {
		_dataMetaRaw, _ = _server.cachedDataMeta[BytesToString (_fingerprintMeta)]
	} else {
		_key := _keyBufferSmall[:0]
		_key = append (_key, NamespaceDataMetadata ...)
		_key = append (_key, ':')
		_key = append (_key, _fingerprintMeta ...)
		if _value, _error := _server.cdbReader.GetWithCdbHash (_key); _error == nil {
			_dataMetaRaw = _value
		} else {
			_server.ServeError (_context, http.StatusInternalServerError, _error, false)
			return
		}
	}
	if _dataMetaRaw == nil {
		if !_server.quiet {
			log.Printf ("[ee] [e8702411]  [cdb.....]  missing data metadata for `%s`!\n", *_requestUriString)
		}
		_server.ServeError (_context, http.StatusInternalServerError, nil, false)
		return
	}
	
	_responseStatus := http.StatusOK
	
	_handleHeader := func (_name []byte, _value []byte) {
			if _name[0] != '!' {
				_responseHeaders.AddBytesKV (_name, _value)
			} else {
				switch BytesToString (_name) {
					case "!Status" :
						if _value, _error := strconv.Atoi (BytesToString (_value)); _error == nil {
							if (_value >= 200) && (_value <= 599) {
								_responseStatus = _value
							} else {
								if !_server.quiet {
									log.Printf ("[ee] [c2f7ec36]  [cdb.....]  invalid data metadata for `%s`!\n", *_requestUriString)
								}
								_responseStatus = http.StatusInternalServerError
								}
						} else {
							if !_server.quiet {
								log.Printf ("[ee] [beedae55]  [cdb.....]  invalid data metadata for `%s`!\n", *_requestUriString)
							}
							_responseStatus = http.StatusInternalServerError
						}
					default :
						if !_server.quiet {
							log.Printf ("[ee] [7acc7d90]  [cdb.....]  invalid data metadata for `%s`!\n", *_requestUriString)
						}
				}
			}
		}
	if _error := MetadataDecodeIterate (_dataMetaRaw, _handleHeader); _error != nil {
		_server.ServeError (_context, http.StatusInternalServerError, _error, false)
		return
	}
	
	if _server.securityHeadersTls {
		_responseHeaders.AddBytesKV (StringToBytes ("Strict-Transport-Security"), StringToBytes ("max-age=31536000"))
		_responseHeaders.AddBytesKV (StringToBytes ("Content-Security-Policy"), StringToBytes ("upgrade-insecure-requests"))
	}
	if _server.securityHeadersEnabled {
		_responseHeaders.AddBytesKV (StringToBytes ("Referrer-Policy"), StringToBytes ("strict-origin-when-cross-origin"))
		_responseHeaders.AddBytesKV (StringToBytes ("X-Content-Type-Options"), StringToBytes ("nosniff"))
		_responseHeaders.AddBytesKV (StringToBytes ("X-XSS-Protection"), StringToBytes ("1; mode=block"))
		_responseHeaders.AddBytesKV (StringToBytes ("X-Frame-Options"), StringToBytes ("sameorigin"))
	}
	
	if _server.http3AltSvc != "" {
		_responseHeaders.AddBytesKV (StringToBytes ("Alt-Svc"), StringToBytes (_server.http3AltSvc))
	}
	
	if _server.debug {
		log.Printf ("[dd] [b15f3cad]  [http-x..]  serving for `%s`...\n", *_requestUriString)
	}
	
	if _server.delay != 0 {
		time.Sleep (_server.delay)
	}
	
	_response.SetStatusCode (_responseStatus)
	_response.SetBodyRaw (_data)
}




func (_server *server) ServeStatic (_context *fasthttp.RequestCtx, _status uint, _data []byte, _contentType string, _contentEncoding string, _cache bool) () {
	
	_response := (*fasthttp.Response) (NoEscape (unsafe.Pointer (&_context.Response)))
	_responseHeaders := (*fasthttp.ResponseHeader) (NoEscape (unsafe.Pointer (&_response.Header)))
	
	_responseHeaders.AddBytesKV (StringToBytes ("Content-Type"), StringToBytes (_contentType))
	_responseHeaders.AddBytesKV (StringToBytes ("Content-Encoding"), StringToBytes (_contentEncoding))
	
	if _cache {
		_responseHeaders.AddBytesKV (StringToBytes ("Cache-Control"), StringToBytes ("public, immutable, max-age=3600"))
	} else {
		_responseHeaders.AddBytesKV (StringToBytes ("Cache-Control"), StringToBytes ("no-store, max-age=0"))
	}
	
	if _server.http3AltSvc != "" {
		_responseHeaders.AddBytesKV (StringToBytes ("Alt-Svc"), StringToBytes (_server.http3AltSvc))
	}
	
	_response.SetStatusCode (int (_status))
	_response.SetBodyRaw (_data)
}


func (_server *server) ServeRedirect (_context *fasthttp.RequestCtx, _status uint, _path []byte, _cache bool) () {
	
	_response := (*fasthttp.Response) (NoEscape (unsafe.Pointer (&_context.Response)))
	_responseHeaders := (*fasthttp.ResponseHeader) (NoEscape (unsafe.Pointer (&_response.Header)))
	
	_responseHeaders.SetCanonical (StringToBytes ("Location"), _path)
	
	if _cache {
		_responseHeaders.AddBytesKV (StringToBytes ("Cache-Control"), StringToBytes ("public, immutable, max-age=3600"))
	} else {
		_responseHeaders.AddBytesKV (StringToBytes ("Cache-Control"), StringToBytes ("no-store, max-age=0"))
	}
	
	if _server.http3AltSvc != "" {
		_responseHeaders.AddBytesKV (StringToBytes ("Alt-Svc"), StringToBytes (_server.http3AltSvc))
	}
	
	_response.SetStatusCode (int (_status))
}


func (_server *server) ServeError (_context *fasthttp.RequestCtx, _status uint, _error error, _cache bool) () {
	
	_response := (*fasthttp.Response) (NoEscape (unsafe.Pointer (&_context.Response)))
	_responseHeaders := (*fasthttp.ResponseHeader) (NoEscape (unsafe.Pointer (&_response.Header)))
	
	_responseHeaders.AddBytesKV (StringToBytes ("Content-Type"), StringToBytes (ErrorBannerContentType))
	_responseHeaders.AddBytesKV (StringToBytes ("Content-Encoding"), StringToBytes (ErrorBannerContentEncoding))
	
	if _cache {
		_responseHeaders.AddBytesKV (StringToBytes ("Cache-Control"), StringToBytes ("public, immutable, max-age=3600"))
	} else {
		_responseHeaders.AddBytesKV (StringToBytes ("Cache-Control"), StringToBytes ("no-store, max-age=0"))
	}
	
	if _banner, _bannerFound := ErrorBannersData[_status]; _bannerFound {
		_response.SetBodyRaw (_banner)
	}
	
	_response.SetStatusCode (int (_status))
	
	if (_error != nil) && !_server.quiet {
		LogError (_error, "[23d6cb35]  [http-x..]  failed handling request!")
	}
}


func (_server *server) ServeDummy (_context *fasthttp.RequestCtx) () {
	_server.ServeStatic (_context, http.StatusOK, DummyData, DummyContentType, DummyContentEncoding, false)
}




func (_server *server) ServeHTTP (_response http.ResponseWriter, _request *http.Request) () {
	
	_requestProtoUnsupported := false
	switch _request.ProtoMajor {
		case 1 :
			_requestProtoUnsupported = _server.http1Disabled || (_request.ProtoMinor < 0) || (_request.ProtoMinor > 1)
			if _server.debug && !_requestProtoUnsupported {
				log.Printf ("[dd] [670e36d4]  [go-http.]  using Go HTTP/1 for `%s`...", _request.URL.Path)
			}
		case 2 :
			_requestProtoUnsupported = _server.http2Disabled || (_request.ProtoMinor != 0)
			if _server.debug && !_requestProtoUnsupported {
				log.Printf ("[dd] [524cd64b]  [go-http.]  using Go HTTP/2 for `%s`...", _request.URL.Path)
			}
		case 3 :
			_requestProtoUnsupported = (_server.quicServer == nil) || (_request.ProtoMinor != 0)
			if _server.debug && !_requestProtoUnsupported {
				log.Printf ("[dd] [be95da51]  [go-http.]  using QUIC HTTP/3 for `%s`...", _request.URL.Path)
			}
		default :
			_requestProtoUnsupported = true
	}
	if _requestProtoUnsupported {
		_request.Close = true
		_response.WriteHeader (http.StatusHTTPVersionNotSupported)
		if !_server.quiet {
			log.Printf ("[ww] [4c44e3c0]  [go-http.]  protocol HTTP/%d not supported for `%s`!", _request.ProtoMajor, _request.URL.Path)
		}
		return
	}
	
	if _server.dummy {
		_responseHeaders := _response.Header ()
		_responseHeaders["Content-Type"] = []string { DummyContentType }
		_responseHeaders["Content-Encoding"] = []string { DummyContentEncoding }
		_responseHeaders["Cache-Control"] = []string { "no-store, max-age=0" }
		_responseHeaders["Date"] = nil
		_response.WriteHeader (http.StatusOK)
		_response.Write (DummyData)
		return
	}
	
	// FIXME:  Reimplemnet this to eliminate the HTTP-encode-followed-by-HTTP-decode!
	
	var _context *fasthttp.RequestCtx
	if _context_0 := _requestContextsPool.Get (); _context_0 != nil {
		_context = _context_0.(*fasthttp.RequestCtx)
	} else {
		_context = new (fasthttp.RequestCtx)
	}
	defer _requestContextsPool.Put (_context)
	
	_context.Request.Reset ()
	_context.Request.Header.SetMethod (_request.Method)
	_context.Request.Header.SetRequestURI (_request.URL.Path)
	
	_context.Response.Reset ()
	
	_server.Serve (_context)
	
	_responseHeaders := NewHttpResponseWriterHeadersBuffer (_context.Response.Header.StatusCode ())
	
	_context.Response.Header.VisitAll (
			func (_key []byte, _value []byte) () {
				switch BytesToString (_key) {
					case "Connection", "Content-Length", "Date" :
						// NOP
					default :
						_responseHeaders.Include (_key, _value)
				}
			})
	
	_responseHeaders.WriteTo (_response)
	
	_response.Write (_context.Response.Body ())
}


var _requestContextsPool sync.Pool




func (_server *server) Printf (_format string, _arguments ... interface{}) () {
	if !_server.quiet {
		log.Printf ("[ee] [47765179]  [fasthttp]  |  " + _format, _arguments ...)
	}
}




func Main () () {
	
	log.SetPrefix (fmt.Sprintf ("[%8d] ", os.Getpid ()))
	
	Main_0 (main_0)
}








func main_0 () (error) {
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	var _bind string
	var _bindTls string
	var _bindTls2 string
	var _bindQuic string
	var _http1Disabled bool
	var _http2Disabled bool
	var _http3AltSvc string
	var _tlsPrivate string
	var _tlsPublic string
	var _archivePath string
	var _archiveInmem bool
	var _archiveMmap bool
	var _archivePreload bool
	var _indexPaths bool
	var _indexDataMeta bool
	var _indexDataContent bool
	var _securityHeadersEnabled bool
	var _securityHeadersTls bool
	var _timeoutDisabled bool
	var _processes uint
	var _threads uint
	var _slave uint
	var _debug bool
	var _quiet bool
	var _dummy bool
	var _delay time.Duration
	var _profileCpu string
	var _profileMem string
	var _limitMemory uint
	
	var _isFirst bool
	var _isMaster bool
	
	
	{
		_flags := flag.NewFlagSet ("kawipiko-server", flag.ContinueOnError)
		
		_flags.Usage = func () () {
			fmt.Fprintf (os.Stderr, "%s", usageText)
		}
		
		_bind_0 := _flags.String ("bind", "", "")
		_bindTls_0 := _flags.String ("bind-tls", "", "")
		_bindTls2_0 := _flags.String ("bind-tls-2", "", "")
		_bindQuic_0 := _flags.String ("bind-quic", "", "")
		_http1Disabled_0 := _flags.Bool ("http1-disable", false, "")
		_http2Disabled_0 := _flags.Bool ("http2-disable", false, "")
		_http3AltSvc_0 := _flags.String ("http3-alt-svc", "", "")
		_archivePath_0 := _flags.String ("archive", "", "")
		_archiveInmem_0 := _flags.Bool ("archive-inmem", false, "")
		_archiveMmap_0 := _flags.Bool ("archive-mmap", false, "")
		_archivePreload_0 := _flags.Bool ("archive-preload", false, "")
		_indexAll_0 := _flags.Bool ("index-all", false, "")
		_indexPaths_0 := _flags.Bool ("index-paths", false, "")
		_indexDataMeta_0 := _flags.Bool ("index-data-meta", false, "")
		_indexDataContent_0 := _flags.Bool ("index-data-content", false, "")
		_timeoutDisabled_0 := _flags.Bool ("timeout-disable", false, "")
		_securityHeadersTls_0 := _flags.Bool ("security-headers-tls", false, "")
		_securityHeadersDisabled_0 := _flags.Bool ("security-headers-disable", false, "")
		_tlsPrivate_0 := _flags.String ("tls-private", "", "")
		_tlsPublic_0 := _flags.String ("tls-public", "", "")
		_tlsBundle_0 := _flags.String ("tls-bundle", "", "")
		_processes_0 := _flags.Uint ("processes", 0, "")
		_threads_0 := _flags.Uint ("threads", 0, "")
		_slave_0 := _flags.Uint ("slave", 0, "")
		_debug_0 := _flags.Bool ("debug", false, "")
		_quiet_0 := _flags.Bool ("quiet", false, "")
		_dummy_0 := _flags.Bool ("dummy", false, "")
		_delay_0 := _flags.Duration ("delay", 0, "")
		_profileCpu_0 := _flags.String ("profile-cpu", "", "")
		_profileMem_0 := _flags.String ("profile-mem", "", "")
		_limitMemory_0 := _flags.Uint ("limit-memory", 0, "")
		
		FlagsParse (_flags, 0, 0)
		
		_bind = *_bind_0
		_bindTls = *_bindTls_0
		_bindTls2 = *_bindTls2_0
		_bindQuic = *_bindQuic_0
		_http1Disabled = *_http1Disabled_0
		_http2Disabled = *_http2Disabled_0
		_http3AltSvc = *_http3AltSvc_0
		_archivePath = *_archivePath_0
		_archiveInmem = *_archiveInmem_0
		_archiveMmap = *_archiveMmap_0
		_archivePreload = *_archivePreload_0
		_indexAll := *_indexAll_0
		_indexPaths = _indexAll || *_indexPaths_0
		_indexDataMeta = _indexAll || *_indexDataMeta_0
		_indexDataContent = _indexAll || *_indexDataContent_0
		_securityHeadersTls = *_securityHeadersTls_0
		_securityHeadersEnabled = ! *_securityHeadersDisabled_0
		_timeoutDisabled = *_timeoutDisabled_0
		_processes = *_processes_0
		_threads = *_threads_0
		_slave = *_slave_0
		_debug = *_debug_0
		_quiet = *_quiet_0 && !_debug
		_dummy = *_dummy_0
		_delay = *_delay_0
		_profileCpu = *_profileCpu_0
		_profileMem = *_profileMem_0
		_limitMemory = *_limitMemory_0
		
		if _slave == 0 {
			_isMaster = true
		}
		if _slave <= 1 {
			_isFirst = true
		}
		
		if (_bind == "") && (_bindTls == "") && (_bindTls2 == "") && (_bindQuic == "") {
			AbortError (nil, "[6edd9512]  expected bind address argument!")
		}
		if (*_tlsBundle_0 != "") && ((*_tlsPrivate_0 != "") || (*_tlsPublic_0 != "")) {
			AbortError (nil, "[717f5f84]  TLS bundle and TLS private/public are mutually exclusive!")
		}
		if (*_tlsBundle_0 != "") {
			_tlsPrivate = *_tlsBundle_0
			_tlsPublic = *_tlsBundle_0
		} else {
			_tlsPrivate = *_tlsPrivate_0
			_tlsPublic = *_tlsPublic_0
		}
		if ((_tlsPrivate != "") && (_tlsPublic == "")) || ((_tlsPublic != "") && (_tlsPrivate == "")) {
			AbortError (nil, "[6e5b42e4]  TLS private/public must be specified together!")
		}
		if ((_tlsPrivate != "") || (_tlsPublic != "")) && ((_bindTls == "") && (_bindTls2 == "") && (_bindQuic == "")) {
			AbortError (nil, "[4e31f251]  TLS certificate specified, but TLS not enabled!")
		}
		
		if _http1Disabled && (_bind != "") {
			AbortError (nil, "[f498816a]  HTTP/1 is mandatory with `--bind`!")
		}
		if _http1Disabled && (_bindTls != "") {
			AbortError (nil, "[f498816a]  HTTP/1 is mandatory with `--bind-tls`!")
		}
		if _http1Disabled && (_bind == "") && (_bindTls == "") && (_bindTls2 == "") {
			log.Printf ("[ww] [6bc56c8e]  HTTP/1 is not available!\n")
		}
		if _http2Disabled && (_bindTls == "") && (_bindTls2 == "") {
			log.Printf ("[ww] [1ed4864c]  HTTP/2 is not available!\n")
		}
		if (_http3AltSvc != "") && (_bindQuic == "") {
			log.Printf ("[ww] [93510d2a]  HTTP/3 Alt-Svc is not available!\n")
		}
		if (_http3AltSvc == "") && (_bindQuic != "") {
			log.Printf ("[ww] [225bda04]  HTTP/3 Alt-Svc is mandatory with QUIC!\n")
		}
		if (_http3AltSvc != "") {
			if strings.HasPrefix (_http3AltSvc, "h3=") {
				// NOP
			} else if _host, _port, _error := net.SplitHostPort (_http3AltSvc); _error == nil {
				_endpoint := net.JoinHostPort (_host, _port)
				_http3AltSvc = fmt.Sprintf ("h3=\"%s\", h3-29=\"%s\"", _endpoint, _endpoint)
			} else {
				AbortError (nil, "[1a5476b1]  HTTP/3 Alt-Svc is invalid!")
			}
			CanonicalHeaderValueRegister (_http3AltSvc)
		}
		
		if !_dummy {
			if _archivePath == "" {
				AbortError (nil, "[eefe1a38]  expected archive file argument!")
			}
			if _archiveInmem && _archiveMmap {
				AbortError (nil, "[a2101041]  archive 'memory-loaded' and 'memory-mapped' are mutually exclusive!")
			}
			if _archiveInmem && _archivePreload {
				log.Printf ("[ww] [3e8a40e4]  archive 'memory-loaded' implies preloading!\n")
				_archivePreload = false
			}
		} else {
			if _isMaster {
				log.Printf ("[ww] [8e014192]  running in dummy mode;  all archive related arguments are ignored!\n")
			}
			_archivePath = ""
			_archiveInmem = false
			_archiveMmap = false
			_archivePreload = false
			_indexAll = false
			_indexPaths = false
			_indexDataMeta = false
			_indexDataContent = false
		}
		if !_dummy && (_delay != 0) {
			if _isMaster {
				log.Printf ("[ww] [e9296c03]  running with a response delay of `%s`!\n", _delay)
			}
		}
		
		if _processes < 1 {
			_processes = 1
		}
		if _threads < 1 {
			_threads = 1
		}
		
		if _processes > 1024 {
			AbortError (nil, "[45736c1d]  maximum number of allowed processes is 1024!")
		}
		if _threads > 1024 {
			AbortError (nil, "[c5df3c8d]  maximum number of allowed threads is 1024!")
		}
		if (_processes * _threads) > 1024 {
			AbortError (nil, "[b0177488]  maximum number of allowed threads in total is 1024!")
		}
		
		if (_limitMemory != 0) && ((_limitMemory > (16 * 1024)) || (_limitMemory < 128)) {
			AbortError (nil, "[2781f54c]  maximum memory limit is between 128 and 16384 MiB!")
		}
		
		if (_processes > 1) && ((_profileCpu != "") || (_profileMem != "")) {
			AbortError (nil, "[cd18d250]  multi-process and profiling are mutually exclusive!")
		}
		if (_processes > 1) && (_bindQuic != "") {
			AbortError (nil, "[d6db77ba]  QUIC is only available with a single process!")
		}
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	runtime.GOMAXPROCS (int (_threads))
	
	debug.SetGCPercent (50)
	debug.SetMaxThreads (int (128 * (_threads / 64 + 1)))
	debug.SetMaxStack (16 * 1024)
	
	
	_httpServerReduceMemory := false
	
	
	if _limitMemory > 0 {
		if !_quiet && _isMaster {
			log.Printf ("[ii] [2c130d70]  limiting memory to %d MiB;\n", _limitMemory)
		}
		{
			_limitMb := (2 * _limitMemory) + (1 * 1024)
			_limit := syscall.Rlimit {
					Cur : uint64 (_limitMb) * 1024 * 1024,
					Max : uint64 (_limitMb) * 1024 * 1024,
				}
			if _error := syscall.Setrlimit (syscall.RLIMIT_AS, &_limit); _error != nil {
				AbortError (_error, "[4da96378]  failed to configure memory limit!")
			}
		}
		{
			_limitMb := _limitMemory
			_limit := syscall.Rlimit {
					Cur : uint64 (_limitMb) * 1024 * 1024,
					Max : uint64 (_limitMb) * 1024 * 1024,
				}
			if _error := syscall.Setrlimit (syscall.RLIMIT_DATA, &_limit); _error != nil {
				AbortError (_error, "[f661b4fe]  failed to configure memory limit!")
			}
		}
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	if _processes > 1 {
		
		log.Printf ("[ii] [06f8c944]  [master..]  sub-processes starting (`%d` processes with `%d` threads each)...\n", _processes, _threads)
		
		_processesJoin := & sync.WaitGroup {}
		
		_processesPid := make ([]*os.Process, _processes)
		
		_processName := os.Args[0]
		_processArguments := make ([]string, 0, len (os.Args))
		if _bind != "" {
			_processArguments = append (_processArguments, "--bind", _bind)
		}
		if _bindTls != "" {
			_processArguments = append (_processArguments, "--bind-tls", _bindTls)
		}
		if _bindTls2 != "" {
			_processArguments = append (_processArguments, "--bind-tls-2", _bindTls2)
		}
		if _http1Disabled {
			_processArguments = append (_processArguments, "--http1-disabled")
		}
		if _http2Disabled {
			_processArguments = append (_processArguments, "--http2-disabled")
		}
		if _http3AltSvc != "" {
			_processArguments = append (_processArguments, "--http3-alt-svc", _http3AltSvc)
		}
		if _archivePath != "" {
			_processArguments = append (_processArguments, "--archive", _archivePath)
		}
		if _archiveInmem {
			_processArguments = append (_processArguments, "--archive-inmem")
		}
		if _archiveMmap {
			_processArguments = append (_processArguments, "--archive-mmap")
		}
		if _archivePreload {
			_processArguments = append (_processArguments, "--archive-preload")
		}
		if _indexPaths {
			_processArguments = append (_processArguments, "--index-paths")
		}
		if _indexDataMeta {
			_processArguments = append (_processArguments, "--index-data-meta")
		}
		if _indexDataContent {
			_processArguments = append (_processArguments, "--index-data-content")
		}
		if _securityHeadersTls {
			_processArguments = append (_processArguments, "--security-headers-tls")
		}
		if !_securityHeadersEnabled {
			_processArguments = append (_processArguments, "--security-headers-disable")
		}
		if _tlsPrivate != "" {
			_processArguments = append (_processArguments, "--tls-private", _tlsPrivate)
		}
		if _tlsPublic != "" {
			_processArguments = append (_processArguments, "--tls-public", _tlsPublic)
		}
		if _timeoutDisabled {
			_processArguments = append (_processArguments, "--timeout-disable")
		}
		if _limitMemory != 0 {
			_processArguments = append (_processArguments, "--limit-memory", fmt.Sprintf ("%d", _limitMemory))
		}
		if _quiet {
			_processArguments = append (_processArguments, "--quiet")
		}
		if _debug {
			_processArguments = append (_processArguments, "--debug")
		}
		if _dummy {
			_processArguments = append (_processArguments, "--dummy")
		}
		_processArguments = append (_processArguments, "--threads", fmt.Sprintf ("%d", _threads))
		
		_processAttributes := & os.ProcAttr {
				Env : []string {},
				Files : []*os.File {
						os.Stdin,
						os.Stdout,
						os.Stderr,
					},
				Sys : nil,
			}
		
		for _processIndex, _ := range _processesPid {
			_processArguments := append ([]string { _processName, "--slave", fmt.Sprintf ("%d", _processIndex + 1) }, _processArguments ...)
			if _processPid, _error := os.StartProcess (_processName, _processArguments, _processAttributes); _error == nil {
				_processesJoin.Add (1)
				_processesPid[_processIndex] = _processPid
				if !_quiet {
					log.Printf ("[ii] [63cb22f8]  [master..]  sub-process `%d` started (with `%d` threads);\n", _processPid.Pid, _threads)
				}
				go func (_index int, _processPid *os.Process) () {
					if _processStatus, _error := _processPid.Wait (); _error == nil {
						if _processStatus.Success () {
							if _debug {
								log.Printf ("[ii] [66b60b81]  [master..]  sub-process `%d` succeeded;\n", _processPid.Pid)
							}
						} else {
							log.Printf ("[ww] [5d25046b]  [master..]  sub-process `%d` failed:  `%s`;  ignoring!\n", _processPid.Pid, _processStatus)
						}
					} else {
						LogError (_error, fmt.Sprintf ("[f1bfc927]  [master..]  failed waiting for sub-process `%d`;  ignoring!", _processPid.Pid))
					}
					_processesPid[_processIndex] = nil
					_processesJoin.Done ()
				} (_processIndex, _processPid)
			} else {
				LogError (_error, "[8892b34d]  [master..]  failed starting sub-process;  ignoring!")
			}
		}
		
		{
			_signals := make (chan os.Signal, 32)
			signal.Notify (_signals, syscall.SIGINT, syscall.SIGTERM)
			go func () () {
				for {
					_signal := <- _signals
					if _debug {
						log.Printf ("[ii] [a9243ecb]  [master..]  signaling sub-processes...\n")
					}
					for _, _processPid := range _processesPid {
						if _processPid != nil {
							if _error := _processPid.Signal (_signal); _error != nil {
								LogError (_error, fmt.Sprintf ("[ab681164]  [master..]  failed signaling sub-process `%d`;  ignoring!", _processPid.Pid))
							}
						}
					}
				}
			} ()
		}
		
		_processesJoin.Wait ()
		
		if !_quiet {
			log.Printf ("[ii] [b949bafc]  [master..]  sub-processes terminated;\n")
		}
		
		return nil
	}
	
	
	if _isMaster {
		log.Printf ("[ii] [6602a54a]  [master..]  starting (with `%d` threads)...\n", _threads)
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	var _cdbReader *cdb.CDB
	if _archivePath != "" {
		
		if !_quiet && (_debug || _isFirst) {
			log.Printf ("[ii] [3b788396]  [cdb.....]  opening archive file `%s`...\n", _archivePath)
		}
		
		var _cdbFile *os.File
		if _cdbFile_0, _error := os.Open (_archivePath); _error == nil {
			_cdbFile = _cdbFile_0
		} else {
			AbortError (_error, "[9e0b5ed3]  [cdb.....]  failed opening archive file!")
		}
		
		var _cdbFileSize int
		{
			var _cdbFileSize_0 int64
			if _cdbFileStat, _error := _cdbFile.Stat (); _error == nil {
				_cdbFileSize_0 = _cdbFileStat.Size ()
			} else {
				AbortError (_error, "[0ccf0a3b]  [cdb.....]  failed opening archive file!")
			}
			if _cdbFileSize_0 < 1024 {
				AbortError (nil, "[6635a2a8]  [cdb.....]  failed opening archive:  file is too small (or empty)!")
			}
			if _cdbFileSize_0 >= (4 * 1024 * 1024 * 1024) {
				AbortError (nil, "[545bf6ce]  [cdb.....]  failed opening archive:  file is too large!")
			}
			_cdbFileSize = int (_cdbFileSize_0)
		}
		
		if _archivePreload {
			if !_quiet {
				log.Printf ("[ii] [13f4ebf7]  [cdb.....]  preloading archive file...\n")
			}
			_buffer := [16 * 1024]byte {}
			_loop : for {
				switch _, _error := _cdbFile.Read (_buffer[:]); _error {
					case io.EOF :
						break _loop
					case nil :
						continue _loop
					default :
						AbortError (_error, "[a1c3b922]  [cdb.....]  failed preloading archive file...")
				}
			}
		}
		
		if _archiveInmem || _archiveMmap {
			
			var _cdbData []byte
			
			if _archiveInmem {
				
				if _debug {
					log.Printf ("[ii] [216e584b]  [cdb.....]  opening memory-loaded archive...\n")
				}
				
				_cdbData = make ([]byte, _cdbFileSize)
				if _, _error := io.ReadFull (_cdbFile, _cdbData); _error != nil {
					AbortError (_error, "[73039784]  [cdb.....]  failed loading archive file!")
				}
				
			} else if _archiveMmap {
				
				if _debug {
					log.Printf ("[ii] [f47fae8a]  [cdb.....]  opening memory-mapped archive...\n")
				}
				
				if _cdbData_0, _error := syscall.Mmap (int (_cdbFile.Fd ()), 0, int (_cdbFileSize), syscall.PROT_READ, syscall.MAP_SHARED); _error == nil {
					_cdbData = _cdbData_0
				} else {
					AbortError (_error, "[c0e2632c]  [cdb.....]  failed mapping archive file!")
				}
				
				if _archivePreload {
					if _debug {
						log.Printf ("[ii] [d96b06c9]  [cdb.....]  preloading memory-loaded archive...\n")
					}
					_buffer := [16 * 1024]byte {}
					_bufferOffset := 0
					for {
						if _bufferOffset == _cdbFileSize {
							break
						}
						_bufferOffset += copy (_buffer[:], _cdbData[_bufferOffset:])
					}
				}
				
			} else {
				panic ("[e4fffcd8]")
			}
			
			if _error := _cdbFile.Close (); _error != nil {
				AbortError (_error, "[5e0449c2]  [cdb.....]  failed closing archive file!")
			}
			
			if _cdbReader_0, _error := cdb.NewFromBufferWithHasher (_cdbData, nil); _error == nil {
				_cdbReader = _cdbReader_0
			} else {
				AbortError (_error, "[27e4813e]  [cdb.....]  failed opening archive!")
			}
			
		} else {
			
			if !_quiet && (_debug || _isFirst) {
				log.Printf ("[ww] [dd697a66]  [cdb.....]  using `read`-based archive (with significant performance impact)!\n")
			}
			
			if _cdbReader_0, _error := cdb.NewFromReaderWithHasher (_cdbFile, nil); _error == nil {
				_cdbReader = _cdbReader_0
			} else {
				AbortError (_error, "[35832022]  [cdb.....]  failed opening archive!")
			}
			
		}
		
		if _schemaVersion, _error := _cdbReader.GetWithCdbHash ([]byte (NamespaceSchemaVersion)); _error == nil {
			if _schemaVersion == nil {
				AbortError (nil, "[09316866]  [cdb.....]  missing archive schema version!")
			} else if string (_schemaVersion) != CurrentSchemaVersion {
				AbortError (nil, "[e6482cf7]  [cdb.....]  invalid archive schema version!")
			}
		} else {
			AbortError (_error, "[87cae197]  [cdb.....]  failed opening archive!")
		}
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	var _cachedFileFingerprints map[string][]byte
	if _indexPaths {
		_cachedFileFingerprints = make (map[string][]byte, 128 * 1024)
	}
	var _cachedDataMeta map[string][]byte
	if _indexDataMeta {
		_cachedDataMeta = make (map[string][]byte, 128 * 1024)
	}
	var _cachedDataContent map[string][]byte
	if _indexDataContent {
		_cachedDataContent = make (map[string][]byte, 128 * 1024)
	}
	
	if _indexPaths || _indexDataMeta || _indexDataContent {
		if !_quiet {
			log.Printf ("[ii] [fa5338fd]  [cdb.....]  indexing archive...\n")
		}
		if _filesIndex, _error := _cdbReader.GetWithCdbHash ([]byte (NamespaceFilesIndex)); _error == nil {
			if _filesIndex != nil {
				_keyBuffer := [1024]byte {}
				for {
					_offset := bytes.IndexByte (_filesIndex, '\n')
					if _offset == 0 {
						continue
					}
					if _offset == -1 {
						break
					}
					_filePath := _filesIndex[: _offset]
					_filesIndex = _filesIndex[_offset + 1 :]
					var _fingerprints []byte
					var _fingerprintContent []byte
					var _fingerprintMeta []byte
					{
						_key := _keyBuffer[:0]
						_key = append (_key, NamespaceFilesContent ...)
						_key = append (_key, ':')
						_key = append (_key, _filePath ...)
						if _fingerprints_0, _error := _cdbReader.GetWithCdbHash (_key); _error == nil {
							if _fingerprints_0 != nil {
								_fingerprints = _fingerprints_0
								_fingerprintsSplit := bytes.IndexByte (_fingerprints, ':')
								if _fingerprintsSplit < 0 {
									AbortError (nil, "[aa6e678f]  [cdb.....]  failed indexing archive!")
								}
								_fingerprintMeta = _fingerprints[:_fingerprintsSplit]
								_fingerprintContent = _fingerprints[_fingerprintsSplit + 1:]
							} else {
								AbortError (_error, "[460b3cf1]  [cdb.....]  failed indexing archive!")
							}
						} else {
							AbortError (_error, "[216f2075]  [cdb.....]  failed indexing archive!")
						}
					}
					if _indexPaths {
						_cachedFileFingerprints[BytesToString (_filePath)] = _fingerprints
					}
					if _indexDataMeta {
						if _, _wasCached := _cachedDataMeta[BytesToString (_fingerprintMeta)]; !_wasCached {
							_key := _keyBuffer[:0]
							_key = append (_key, NamespaceDataMetadata ...)
							_key = append (_key, ':')
							_key = append (_key, _fingerprintMeta ...)
							if _dataMeta, _error := _cdbReader.GetWithCdbHash (_key); _error == nil {
								if _dataMeta != nil {
									_cachedDataMeta[BytesToString (_fingerprintMeta)] = _dataMeta
								} else {
									AbortError (_error, "[6df556bf]  [cdb.....]  failed indexing archive!")
								}
							} else {
								AbortError (_error, "[0d730134]  [cdb.....]  failed indexing archive!")
							}
						}
					}
					if _indexDataContent {
						if _, _wasCached := _cachedDataContent[BytesToString (_fingerprintContent)]; !_wasCached {
							_key := _keyBuffer[:0]
							_key = append (_key, NamespaceDataContent ...)
							_key = append (_key, ':')
							_key = append (_key, _fingerprintContent ...)
							if _dataContent, _error := _cdbReader.GetWithCdbHash (_key); _error == nil {
								if _dataContent != nil {
									_cachedDataContent[BytesToString (_fingerprintContent)] = _dataContent
								} else {
									AbortError (_error, "[4e27fe46]  [cdb.....]  failed indexing archive!")
								}
							} else {
								AbortError (_error, "[532845ad]  [cdb.....]  failed indexing archive!")
							}
						}
					}
				}
			} else {
				log.Printf ("[ww] [30314f31]  [cdb.....]  missing archive files index;  ignoring!\n")
			}
		} else {
			AbortError (_error, "[82299b3d]  [cdb.....]  failed indexing arcdive!")
		}
	}
	
	if _indexPaths && _indexDataMeta && _indexDataContent {
		if _error := _cdbReader.Close (); _error == nil {
			_cdbReader = nil
		} else {
			AbortError (_error, "[d7aa79e1]  [cdb.....]  failed closing archive!")
		}
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	if _profileCpu != "" {
		log.Printf ("[ii] [70c210f3]  [pprof...]  profiling CPU to `%s`...\n", _profileCpu)
		_stream, _error := os.Create (_profileCpu)
		if _error != nil {
			AbortError (_error, "[fd4e0009]  [pprof...]  failed opening CPU profile!")
		}
		_error = pprof.StartCPUProfile (_stream)
		if _error != nil {
			AbortError (_error, "[ac721629]  [pprof...]  failed starting CPU profile!")
		}
		defer pprof.StopCPUProfile ()
	}
	if _profileMem != "" {
		log.Printf ("[ii] [9196ee90]  [pprof...]  profiling MEM to `%s`...\n", _profileMem)
		_stream, _error := os.Create (_profileMem)
		if _error != nil {
			AbortError (_error, "[907d08b5]  [pprof...]  failed opening MEM profile!")
		}
		_profile := pprof.Lookup ("heap")
		defer func () () {
			runtime.GC ()
			if _profile != nil {
				if _error := _profile.WriteTo (_stream, 0); _error != nil {
					AbortError (_error, "[4b1e5112]  [pprof...]  failed writing MEM profile!")
				}
			} else {
				AbortError (nil, "[385dc8f0]  [pprof...]  failed loading MEM profile!")
			}
			_stream.Close ()
		} ()
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	_server := & server {
			cdbReader : _cdbReader,
			cachedFileFingerprints : _cachedFileFingerprints,
			cachedDataMeta : _cachedDataMeta,
			cachedDataContent : _cachedDataContent,
			securityHeadersTls : _securityHeadersTls,
			securityHeadersEnabled : _securityHeadersEnabled,
			http1Disabled : _http1Disabled,
			http2Disabled : _http2Disabled,
			http3AltSvc : _http3AltSvc,
			quiet : _quiet,
			debug : _debug,
			dummy : _dummy,
			delay : _delay,
		}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	_tlsConfig := & tls.Config {
			Certificates : nil,
			MinVersion : tls.VersionTLS12,
			MaxVersion : tls.VersionTLS13,
			CipherSuites : []uint16 {
					// NOTE:  https://wiki.mozilla.org/Security/Server_Side_TLS#Modern_compatibility
					// NOTE:  https://github.com/golang/go/issues/29349
					// NOTE:  TLSv1.3
					tls.TLS_CHACHA20_POLY1305_SHA256,
					tls.TLS_AES_128_GCM_SHA256,
					tls.TLS_AES_256_GCM_SHA384,
					// NOTE:  TLSv1.2
					// NOTE:  https://datatracker.ietf.org/doc/html/rfc7540#section-9.2.2
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				},
			Renegotiation : tls.RenegotiateNever,
			SessionTicketsDisabled : true,
			DynamicRecordSizingDisabled : true,
			NextProtos : []string { "http/1.1", "http/1.0" },
		}
	
	if (_bindTls != "") || (_bindTls2 != "") {
		if _tlsPrivate != "" {
			if _certificate, _error := tls.LoadX509KeyPair (_tlsPublic, _tlsPrivate); _error == nil {
				_tlsConfig.Certificates = append (_tlsConfig.Certificates, _certificate)
			} else {
				AbortError (_error, "[ecdf443d]  [tls.....]  failed loading TLS certificate!")
			}
		}
		if len (_tlsConfig.Certificates) == 0 {
			if !_quiet {
				log.Printf ("[ii] [344ba198]  [tls.....]  no TLS certificate specified;  using self-signed!\n")
			}
			if _certificate, _error := tls.X509KeyPair ([]byte (DefaultTlsCertificatePublic), []byte (DefaultTlsCertificatePrivate)); _error == nil {
				_tlsConfig.Certificates = append (_tlsConfig.Certificates, _certificate)
			} else {
				AbortError (_error, "[98ba6d23]  [tls.....]  failed parsing TLS certificate!")
			}
		}
	}
	
	
	
	
	_httpServer := & fasthttp.Server {
			
			Name : "kawipiko",
			Handler : _server.Serve,
			GetOnly : true,
			
			NoDefaultServerHeader : true,
			NoDefaultContentType : true,
			NoDefaultDate : true,
			DisableHeaderNamesNormalizing : true,
			
			Concurrency : 16 * 1024 + 128,
			MaxRequestsPerConn : 256 * 1024,
			
			ReadBufferSize : 16 * 1024,
			WriteBufferSize : 16 * 1024,
			MaxRequestBodySize : 16 * 1024,
			
			ReadTimeout : 30 * time.Second,
			WriteTimeout : 30 * time.Second,
			IdleTimeout : 360 * time.Second,
			
			TCPKeepalive : true,
			TCPKeepalivePeriod : 60 * time.Second,
			
			ReduceMemoryUsage : _httpServerReduceMemory,
			
			CloseOnShutdown : true,
			DisableKeepalive : false,
			
			Logger : _server,
			
		}
	
	
	_httpsServer := & fasthttp.Server {}
	*_httpsServer = *_httpServer
	
	
	
	
	_https2Server := & http.Server {
			
			Handler : _server,
			TLSConfig : nil,
			
			MaxHeaderBytes : _httpsServer.ReadBufferSize,
			
			ReadTimeout : _httpsServer.ReadTimeout,
			ReadHeaderTimeout : _httpsServer.ReadTimeout,
			WriteTimeout : _httpsServer.WriteTimeout,
			IdleTimeout : _httpsServer.IdleTimeout,
			
		}
	
	_tls2Config := _tlsConfig.Clone ()
	if !_http1Disabled && !_http2Disabled {
		_tls2Config.NextProtos = []string { "h2", "http/1.1", "http/1.0" }
	} else if !_http1Disabled {
		_tls2Config.NextProtos = []string { "http/1.1", "http/1.0" }
	} else if !_http2Disabled {
		_tls2Config.NextProtos = []string { "h2" }
	} else if _bindQuic != "" {
		// NOP
	} else {
		panic ("[1b618ffe]")
	}
	
	if !_quiet {
		_https2Server.ErrorLog = log.New (os.Stderr, log.Prefix () + "[ee] [f734edc4]  [go-http.]  |  ", 0)
	} else {
		_https2Server.ErrorLog = log.New (ioutil.Discard, "", 0)
	}
	
	
	
	
	_quicServer := & http3.Server {}
	
	_quicServer.Server = & http.Server {
			
			Handler : _server,
			TLSConfig : nil,
			
		}
	
	_quicTlsConfig := _tlsConfig.Clone ()
	_quicTlsConfig.NextProtos = []string { "h3", "h3-29" }
	_quicServer.Server.TLSConfig = _quicTlsConfig
	
	_quicServer.QuicConfig = & quic.Config {
			
			Versions : []quic.VersionNumber {
					quic.Version1,
					quic.VersionDraft29,
				},
			
			HandshakeIdleTimeout : 6 * time.Second,
			MaxIdleTimeout : _httpsServer.IdleTimeout,
			
			MaxIncomingStreams : 1024,
			MaxIncomingUniStreams : 1024,
			
			InitialConnectionReceiveWindow : 1 * 1024 * 1024,
			MaxConnectionReceiveWindow : 4 * 1024 * 1024,
			
			InitialStreamReceiveWindow : 512 * 1024,
			MaxStreamReceiveWindow : 2 * 1024 * 1024,
			KeepAlive : true,
			
		}
	
	if !_quiet {
		_quicServer.Server.ErrorLog = log.New (os.Stderr, log.Prefix () + "[ee] [a6af7354]  [quic-h3.]  |  ", 0)
	} else {
		_quicServer.Server.ErrorLog = log.New (ioutil.Discard, "", 0)
	}
	
	
	
	
	if _timeoutDisabled {
		
		_httpServer.ReadTimeout = 0
		_httpServer.WriteTimeout = 0
		_httpServer.IdleTimeout = 0
		
		_httpsServer.ReadTimeout = 0
		_httpsServer.WriteTimeout = 0
		_httpsServer.IdleTimeout = 0
		
		_https2Server.ReadTimeout = 0
		_https2Server.ReadHeaderTimeout = 0
		_https2Server.WriteTimeout = 0
		_https2Server.IdleTimeout = 0
		
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	if !_quiet && (_debug || _isFirst) {
		if _bind != "" {
			log.Printf ("[ii] [f11e4e37]  [bind-0..]  listening on `http://%s/` (using FastHTTP supporting HTTP/1.1, HTTP/1.0);\n", _bind)
		}
		if _bindTls != "" {
			if !_http1Disabled && (!_http2Disabled && _bindTls2 == "") {
				log.Printf ("[ii] [21f050c3]  [bind-1..]  listening on `https://%s/` (using FastHTTP supporting TLS with HTTP/1.1, HTTP/1.0, and HTTP/2 split);\n", _bindTls)
			} else if !_http1Disabled {
				log.Printf ("[ii] [21f050c3]  [bind-1..]  listening on `https://%s/` (using FastHTTP supporting TLS with HTTP/1.1, HTTP/1.0);\n", _bindTls)
			} else {
				panic ("[fc754170]")
			}
		}
		if _bindTls2 != "" {
			if !_http1Disabled && !_http2Disabled {
				log.Printf ("[ii] [e7f03c99]  [bind-2..]  listening on `https://%s/` (using Go HTTP supporting TLS with HTTP/2, HTTP/1.1, HTTP/1.0);\n", _bindTls2)
			} else if !_http1Disabled {
				log.Printf ("[ii] [477583ad]  [bind-2..]  listening on `https://%s/` (using Go HTTP supporting TLS with HTTP/1.1, HTTP/1.0 only);\n", _bindTls2)
			} else if !_http2Disabled {
				log.Printf ("[ii] [7d2c7ddb]  [bind-2..]  listening on `https://%s/` (using Go HTTP supporting TLS with HTTP/2 only);\n", _bindTls2)
			} else {
				panic ("[d784a82c]")
			}
		}
		if _bindQuic != "" {
			log.Printf ("[ii] [b958617a]  [bind-3..]  listening on `https://%s/` (using QUIC supporting TLS with HTTP/3 only);", _bindQuic)
		}
	}
	
	
	var _httpListener net.Listener
	if _bind != "" {
		if _listener_0, _error := reuseport.Listen ("tcp4", _bind); _error == nil {
			_httpListener = _listener_0
		} else {
			AbortError (_error, "[d5f51e9f]  [bind-0..]  failed creating TCP listener!")
		}
	}
	
	var _httpsListener net.Listener
	if _bindTls != "" {
		if _listener_0, _error := reuseport.Listen ("tcp4", _bindTls); _error == nil {
			_httpsListener = _listener_0
		} else {
			AbortError (_error, "[e35cc693]  [bind-1..]  failed creating TCP listener!")
		}
	}
	
	var _https2Listener net.Listener
	if _bindTls2 != "" {
		if _listener_0, _error := reuseport.Listen ("tcp4", _bindTls2); _error == nil {
			_https2Listener = _listener_0
		} else {
			AbortError (_error, "[63567445]  [bind-2..]  failed creating TCP listener!")
		}
	}
	
	var _quicListener net.PacketConn
	if _bindQuic != "" {
		if _listener_0, _error := net.ListenPacket ("udp4", _bindQuic); _error == nil {
			_quicListener = _listener_0
		} else {
			AbortError (_error, "[3b1bfc15]  [bind-3..]  failed creating UDP listener!")
		}
	}
	
	
	var _splitListenerClose func () ()
	if (_httpsListener != nil) && (_https2Listener == nil) && !_http2Disabled {
		log.Printf ("[ii] [1098a405]  [bind-1..]  listening on `https://%s/` (using Go HTTP supporting only HTTP/2 split);\n", _bindTls)
		_tlsConfig.NextProtos = append ([]string { "h2" }, _tlsConfig.NextProtos ...)
		if !_quiet {
			log.Printf ("[ii] [ba970bbb]  [bind-1..]  advertising TLS next protocols: %s", _tlsConfig.NextProtos)
		}
		_tlsListener := tls.NewListener (_httpsListener, _tlsConfig)
		_httpsListener_0 := & splitListener {
				listener : _tlsListener,
				queue : make (chan net.Conn),
			}
		_https2Listener_0 := & splitListener {
				listener : _tlsListener,
				queue : make (chan net.Conn),
			}
		_splitListenerClose = func () () {
			if _error := _tlsListener.Close (); _error != nil {
				LogError (_error, "[a5bce477]  [bind-1..]  failed closing TLS listener!")
			}
		}
		go func () () {
				for {
					if _connection_0, _error := _tlsListener.Accept (); _error == nil {
						go func () () {
							_connection := _connection_0.(*tls.Conn)
							if _error := _connection.Handshake (); _error != nil {
								if !_quiet {
									LogError (_error, "[d1c3dba3]  [bind-1..]  failed negotiating TLS connection!")
								}
							}
							_protocol := _connection.ConnectionState () .NegotiatedProtocol
							if _protocol == "h2" {
								if _debug {
									log.Printf ("[dd] [df9f3e7e]  [bind-1..]  dispatching HTTP/2 TLS connection!\n")
								}
								_https2Listener_0.queue <- _connection
							} else if (_protocol == "http/1.1") || (_protocol == "http/1.0") || (_protocol == "") {
								if _debug {
									log.Printf ("[dd] [d534c361]  [bind-1..]  dispatching HTTP/1.x TLS connection!\n")
								}
								_httpsListener_0.queue <- _connection
							} else {
								if !_quiet {
									log.Printf ("[ww] [5cc0ebde]  [bind-1..]  unknown TLS protocol `%s`!\n", _protocol)
								}
								_connection.Close ()
							}
						} ()
					} else if strings.Contains (_error.Error (), "use of closed network connection") {
						break
					} else {
						LogError (_error, "[04b6637f]  [bind-1..]  failed accepting TLS connection!")
						break
					}
				}
			} ()
		_httpsListener = _httpsListener_0
		_https2Listener = _https2Listener_0
	} else {
		if _httpsListener != nil {
			if !_quiet {
				log.Printf ("[ii] [ceed854a]  [bind-1..]  advertising TLS next protocols: %s", _tlsConfig.NextProtos)
			}
			_httpsListener = tls.NewListener (_httpsListener, _tlsConfig)
		}
		if _https2Listener != nil {
			if !_quiet {
				log.Printf ("[ii] [8b97c977]  [bind-2..]  advertising TLS next protocols: %s", _tls2Config.NextProtos)
			}
			_https2Listener = tls.NewListener (_https2Listener, _tls2Config)
		}
	}
	
	
	if _quicListener != nil {
		if !_quiet {
			log.Printf ("[ii] [22feb826]  [bind-3..]  advertising TLS next protocols: %s", _quicTlsConfig.NextProtos)
		}
	}
	
	
	if _httpListener != nil {
		_server.httpServer = _httpServer
	}
	if _httpsListener != nil {
		_server.httpsServer = _httpsServer
	}
	if _https2Listener != nil {
		_server.https2Server = _https2Server
	}
	if _quicListener != nil {
		_server.quicServer = _quicServer
	}
	
	_httpServer = nil
	_httpsServer = nil
	_https2Server = nil
	_quicServer = nil
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	var _waiter sync.WaitGroup
	
	if _server.httpServer != nil {
		_waiter.Add (1)
		go func () () {
			defer _waiter.Done ()
			if !_quiet {
				log.Printf ("[ii] [f2061f1b]  [fasthttp]  starting FastHTTP server...\n")
			}
			if _error := _server.httpServer.Serve (_httpListener); _error != nil {
				AbortError (_error, "[44f45c67]  [fasthttp]  failed executing server!")
			}
			if !_quiet {
				log.Printf ("[ii] [aca4a14f]  [fasthttp]  stopped FastHTTP server;\n")
			}
		} ()
	}
	
	if _server.httpsServer != nil {
		_waiter.Add (1)
		go func () () {
			defer _waiter.Done ()
			if !_quiet {
				log.Printf ("[ii] [83cb1f6f]  [fasthttp]  starting FastHTTP server (for TLS)...\n")
			}
			if _error := _server.httpsServer.Serve (_httpsListener); _error != nil {
				AbortError (_error, "[b2d50852]  [fasthttp]  failed executing server!")
			}
			if !_quiet {
				log.Printf ("[ii] [ee4180b7]  [fasthttp]  stopped FastHTTP server (for TLS);\n")
			}
		} ()
	}
	
	if _server.https2Server != nil {
		_waiter.Add (1)
		go func () () {
			defer _waiter.Done ()
			if !_quiet {
				log.Printf ("[ii] [46ec2e41]  [go-http.]  starting Go HTTP server...\n")
			}
			if _error := _server.https2Server.Serve (_https2Listener); (_error != nil) && (_error != http.ErrServerClosed) {
				AbortError (_error, "[9f6d28f4]  [go-http.]  failed executing server!")
			}
			if !_quiet {
				log.Printf ("[ii] [9a487770]  [go-http.]  stopped Go HTTP server;\n")
			}
		} ()
	}
	
	if _server.quicServer != nil {
		_waiter.Add (1)
		go func () () {
			defer _waiter.Done ()
			if !_quiet {
				log.Printf ("[ii] [4cf834b0]  [quic-h3.]  starting QUIC server...\n")
			}
			if _error := _server.quicServer.Serve (_quicListener); (_error != nil) && (_error.Error () != "server closed") {
				AbortError (_error, "[73e700c5]  [quic-h3.]  failed executing server!")
			}
			if !_quiet {
				log.Printf ("[ii] [0a9d72e9]  [quic-h3.]  stopped QUIC server;\n")
			}
		} ()
	}
	
	{
		_waiter.Add (1)
		_signals := make (chan os.Signal, 32)
		signal.Notify (_signals, syscall.SIGINT, syscall.SIGTERM)
		go func () () {
			defer _waiter.Done ()
			<- _signals
			if !_quiet {
				log.Printf ("[ii] [691cb695]  [shutdown]  terminating...\n")
			}
			if _server.httpServer != nil {
				_waiter.Add (1)
				go func () () {
					defer _waiter.Done ()
					if !_quiet {
						log.Printf ("[ii] [8eea3f63]  [fasthttp]  stopping FastHTTP server...\n")
					}
					_server.httpServer.Shutdown ()
				} ()
			}
			if _splitListenerClose != nil {
				_waiter.Add (1)
				go func () () {
					defer _waiter.Done ()
					_splitListenerClose ()
				} ()
			}
			if _server.httpsServer != nil {
				_waiter.Add (1)
				go func () () {
					defer _waiter.Done ()
					if !_quiet {
						log.Printf ("[ii] [ff651007]  [fasthttp]  stopping FastHTTP server (for TLS)...\n")
					}
					_server.httpsServer.Shutdown ()
				} ()
			}
			if _server.https2Server != nil {
				_waiter.Add (1)
				go func () () {
					defer _waiter.Done ()
					if !_quiet {
						log.Printf ("[ii] [9ae5a25b]  [go-http.]  stopping Go HTTP server...\n")
					}
					_server.https2Server.Shutdown (context.TODO ())
				} ()
			}
			if _server.quicServer != nil {
				_waiter.Add (1)
				go func () () {
					defer _waiter.Done ()
					if !_quiet {
						log.Printf ("[ii] [41dab8c2]  [quic-h3.]  stopping QUIC server...\n")
					}
					_server.quicServer.CloseGracefully (1 * time.Second)
					time.Sleep (1 * time.Second)
					_server.quicServer.Close ()
				} ()
			}
			if true {
				go func () () {
					time.Sleep (6 * time.Second)
					AbortError (nil, "[827c672c]  [shutdown]  forced terminated!")
				} ()
			}
		} ()
	}
	
	_waiter.Wait ()
	
	if !_quiet {
		defer log.Printf ("[ii] [a49175db]  [shutdown]  terminated!\n")
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	if _cdbReader != nil {
		_server.cdbReader = nil
		if _error := _cdbReader.Close (); _error != nil {
			AbortError (_error, "[a1031c39]  [cdb.....]  failed closing archive!")
		}
	}
	
	
	
	
	// --------------------------------------------------------------------------------
	// --------------------------------------------------------------------------------
	
	
	
	
	return nil
}




type splitListener struct {
	listener net.Listener
	queue chan net.Conn
}

func (_listener *splitListener) Accept () (net.Conn, error) {
	if _connection, _ok := <- _listener.queue; _ok {
		return _connection, nil
	} else {
		return nil, io.EOF
	}
}

func (_listener *splitListener) Close () (error) {
	close (_listener.queue)
	return nil
}

func (_listener *splitListener) Addr () (net.Addr) {
	if _listener.listener != nil {
		return _listener.listener.Addr ()
	} else {
		return nil
	}
}




//go:embed usage.txt
var usageText string

