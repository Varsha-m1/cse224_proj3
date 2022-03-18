package tritonhttp

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Request struct {
	Method string // e.g. "GET"
	URL    string // e.g. "/path/to/a/file"
	Proto  string // e.g. "HTTP/1.1"

	// Header stores misc headers excluding "Host" and "Connection",
	// which are stored in special fields below.
	// Header keys are case-incensitive, and should be stored
	// in the canonical format in this map.
	Header map[string]string

	Host  string // determine from the "Host" header
	Close bool   // determine from the "Connection" header
}

// ReadRequest tries to read the next valid request from br.
//
// If it succeeds, it returns the valid request read. In this case,
// bytesReceived should be true, and err should be nil.
//
// If an error occurs during the reading, it returns the error,
// and a nil request. In this case, bytesReceived indicates whether or not
// some bytes are received before the error occurs. This is useful to determine
// the timeout with partial request received condition.
func ReadRequest(br *bufio.Reader) (req *Request, bytesReceived bool, err error) {
	req = &Request{}

	// Read start line
	line, err := ReadLine(br)
	if err != nil {
		return nil, false, err
	}

	method, url, proto, err := parseRequestLine(line)
	if err != nil {
		return nil, false, badStringError("malformed start line", line)
	}

	if !validMethod(method) {
		return nil, false, badStringError("invalid method", method)
	}

	if !validProto(proto) {
		return nil, false, badStringError("invalid proto", proto)
	}

	if !validUrl(url) {
		return nil, false, badStringError("invalid url", url)
	}

	req.Method = method
	req.URL = url
	req.Proto = proto

	m := make(map[string]string)

	for {
		line, err := ReadLine(br)
		if err != nil {
			if line == "" {
				return nil, false, err
			}

			req.Close = false
			return req, false, badStringError("malformed body", line)
		}
		if line == "" {
			break
		}

		key, value, err := getKeyValue(line)

		if (key == "" && value != "") || invalidVal(value) || invalidKey(key) {
			req.Close = false
			return req, false, badStringError("malformed body key val", "")
		}
		if err != nil {
			return nil, false, err
		}
		key = CanonicalHeaderKey(key)
		if key == "Host" {
			req.Host = value
		} else if strings.EqualFold(key, "Connection") {
			if value == "close" {
				req.Close = true
			} else {
				continue
			}

		} else {
			m[key] = value
		}
	}

	req.Header = m
	return req, true, nil
}

func badStringError(what, val string) error {
	return errors.New(fmt.Sprintf("%s %q", what, val))
}

func invalidValue(val string) bool {
	if strings.Contains(val, "\r\n") || (val != "" && string(val[0]) == string(" ")) {
		return true
	}

	return false
}

func validMethod(method string) bool {
	return method == "GET"
}

func validProto(proto string) bool {
	return proto == "HTTP/1.1"
}

func validUrl(url string) bool {
	return string(url[0]) == string("/")
}

func invalidKey(key string) bool {
	isAlpha := regexp.MustCompile(`^[A-Za-z0-9-]+$`).MatchString
	return !isAlpha(key)
}

func parseRequestLine(line string) (string, string, string, error) {
	fields := strings.SplitN(line, " ", 3)
	if len(fields) != 3 {
		return "", "", "", fmt.Errorf("could not parse the request line, got fields %v", fields)
	}
	return fields[0], fields[1], fields[2], nil
}

func getKeyValue(line string) (string, string, error) {
	fields := strings.SplitN(line, ":", 2)
	if len(fields) != 2 {
		return "", "", fmt.Errorf("could not parse the request line, got fields %v", fields)
	}
	return strings.TrimLeft(fields[0], " "), strings.TrimLeft(fields[1], " "), nil
}
