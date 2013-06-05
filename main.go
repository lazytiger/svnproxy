package main

import (
  "bytes"
	"encoding/base64"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

const (
	InternalServerError     = "Internal Server Error"
	UnauthorizedAccessError = "Unauthorized Access"
)

type SvnAdmin struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	ListenAddr   string
	ProxyHost    string
	server       http.Server
	Auther       SvnAuther
	Prefix       string
	prefixLen    int
}

var (
	admin *SvnAdmin
)

var (
	MethodMap = map[string]uint{
		"HEAD":       0x00001, //false,
		"PROPFIND":   0x00002, //false,
		"PROPPATCH":  0x00004, //true,
		"MKCOL":      0x00008, //true,
		"MKCALENDAR": 0x00010, //true,
		"COPY":       0x00020, //true,
		"MOVE":       0x00040, //true,
		"LOCK":       0x00080, //true,
		"UNLOCK":     0x00100, //true,
		"DELETE":     0x00200, //true,
		"GET":        0x00400, //false,
		"OPTIONS":    0x00800, //false,
		"POST":       0x01000, //true,
		"PUT":        0x02000, //true,
		"TRACE":      0x04000, //false,
		"ACL":        0x08000, //true,
		"CONNECT":    0x10000, //true,
		"REPORT":     0x20000, //false,
		"MKACTIVITY": 0x40000, //true,
		"MERGE":      0x80000, //true,
	}
)

type SvnAuther interface {
	CanAccess(username string, path string, method string) bool
}

func addHeaderCheckRedirect(req *http.Request, via []*http.Request) error {
	return errors.New(req.URL.RequestURI())
}

func getAuthFromBasicRealm(realm string) []string {
	realms := strings.SplitAfterN(realm, " ", 2)
	if len(realms) != 2 {
		return nil
	}
	source := bytes.NewReader([]byte(realms[1]))
	dest := base64.NewDecoder(base64.StdEncoding, source)
	destBytes, err := ioutil.ReadAll(dest)
	if err != nil {
		return nil
	}
	realms = strings.SplitN(string(destBytes), ":", 2)
	if len(realms) != 2 {
		return nil
	}
	return realms
}

func (admin *SvnAdmin) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	defer func() {
		err := recover()
		if err != nil {
			stack := string(debug.Stack())
			log.Printf("unhandled panic found, stack:%s", stack)
			writer.WriteHeader(http.StatusInternalServerError)
			writer.Write([]byte(UnauthorizedAccessError))
		}
	}()

	log.Printf("path:%s", request.RequestURI)
	realm := request.Header.Get("Authorization")
	if realm != "" {
		//log.Printf("authorization found:%s", realm)
		auth := getAuthFromBasicRealm(realm)
		if auth != nil {
			username := auth[0]
			method := request.Method
			path := request.RequestURI
			for {
				opath := path
				path = strings.Replace(path, "//", "/", -1)
				if len(opath) == len(path) {
					break
				}
			}
			if !strings.HasPrefix(path, admin.Prefix) {
				writer.WriteHeader(http.StatusForbidden)
				writer.Write([]byte(UnauthorizedAccessError))
				return
			}

			path = path[admin.prefixLen:]
			log.Printf("%s %s on %s from %s", username, method, path, request.RemoteAddr)
			if !admin.Auther.CanAccess(username, method, path) {
				writer.WriteHeader(http.StatusForbidden)
				writer.Write([]byte(UnauthorizedAccessError))
				return
			}
		}
	}

	requestURL := "http://" + admin.ProxyHost + request.RequestURI
	req, err := http.NewRequest(request.Method, requestURL, request.Body)
	if err != nil {
		log.Printf("make request failed:%s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		writer.Write([]byte(UnauthorizedAccessError))
		return
	}

	req.Header = request.Header
	req.Header.Set("HTTP_X_FORWARDED_FOR", request.RemoteAddr)
	proxy := &http.Client{}
	proxy.CheckRedirect = addHeaderCheckRedirect
	resp, err := proxy.Do(req)
	if err != nil {
		nerr, ok := err.(*url.Error)
		if !ok {
			log.Printf("proxy return err:%s", err.Error())
			writer.WriteHeader(http.StatusInternalServerError)
			writer.Write([]byte(InternalServerError))
		} else {
			log.Printf("proxy redirect to path:%s", nerr.Err.Error())
			writer.Header().Set("Location", nerr.Err.Error())
			writer.WriteHeader(http.StatusFound)
		}
		return
	}
	log.Printf("proxy return:%s", resp.Status)

	for k, vs := range resp.Header {
		for _, v := range vs {
			writer.Header().Add(k, v)
		}
	}
	data, _ := ioutil.ReadAll(resp.Body)
	writer.WriteHeader(resp.StatusCode)
	writer.Write(data)
}

func (admin *SvnAdmin) Start() {
	admin.server.Addr = admin.ListenAddr
	admin.server.ReadTimeout = admin.ReadTimeout
	admin.server.WriteTimeout = admin.WriteTimeout
	log.Printf("server started")
	err := admin.server.ListenAndServe()
	if err != nil {
		log.Panic(err)
	}
	log.Printf("server stoped")
}

func NewSvnAdmin() *SvnAdmin {
	admin := &SvnAdmin{
		ReadTimeout:  1000000000,
		WriteTimeout: 1000000000,
	}
	admin.server.Handler = admin
	return admin
}

func main() {

	mysqlConnStr := ""
	checkInterval := int64(0)
	log.SetFlags(log.Ldate | log.Lshortfile | log.Ltime | log.Lmicroseconds)
	admin = NewSvnAdmin()
	flag.StringVar(&admin.ListenAddr, "listen", ":80", "监听的地址")
	flag.StringVar(&admin.ProxyHost, "svn_host", "", "代理的svn地址")
	flag.StringVar(&admin.Prefix, "svn_prefix", "/svn/root", "svn请求的前缀")
	flag.StringVar(&mysqlConnStr, "db_str", "", "数据库地址,例如tcp:host:port*db_name/username/passwd")
	flag.Int64Var(&checkInterval, "check_time", 10000000000, "检查更新周期")
	flag.Parse()
	if admin.ProxyHost == "" || mysqlConnStr == "" {
		flag.Usage()
		return
	}

	admin.prefixLen = len([]byte(admin.Prefix))
	ignoreList := []string{
		"/favicon.ico",
	}
	admin.Auther = newSvnAuther(time.Duration(checkInterval), mysqlConnStr, ignoreList)
	admin.Start()
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
