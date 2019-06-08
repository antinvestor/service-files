package utils

import (
	"net"
	"net/http"
)

func GetIp(r *http.Request) string  {
	sourceIp := r.Header.Get("X-FORWARDED-FOR")
	if sourceIp == ""{
		sourceIp,_,_ = net.SplitHostPort(r.RemoteAddr)
	}

	return sourceIp
}
