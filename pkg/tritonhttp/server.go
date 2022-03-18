package tritonhttp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	responseProto = "HTTP/1.1"

	statusOK               = 200
	statusMethodNotAllowed = 400
	statusMethodNotFound   = 404
)

type Server struct {
	// Addr specifies the TCP address for the server to listen on,
	// in the form "host:port". It shall be passed to net.Listen()
	// during ListenAndServe().
	Addr string // e.g. ":0"

	// DocRoot specifies the path to the directory to serve static files from.
	DocRoot string
}

// ListenAndServe listens on the TCP network address s.Addr and then
// handles requests on incoming connections.
func (s *Server) ListenAndServe() error {
	// Hint: call HandleConnection
	if err := s.ValidateServerSetup(); err != nil {
		return fmt.Errorf("server is not setup correctly %v", err)
	}

	// server should now start to listen on the configured address
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}

	// making sure the listener is closed when we exit
	defer func() {
		err = ln.Close()
		if err != nil {
			fmt.Println("error in closing listener", err)
		}
	}()

	// accept connections forever
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		fmt.Println("accepted connection", conn.RemoteAddr())
		go s.HandleConnection(conn)
	}
}

func (s *Server) ValidateServerSetup() error {
	// Validating the doc root of the server
	directory, err := os.Stat(s.DocRoot)

	if os.IsNotExist(err) {
		return err
	}

	if !directory.IsDir() {
		return errors.New("DocRoot not a directory")
	}

	return nil
}

// HandleConnection reads requests from the accepted conn and handles them.
func (s *Server) HandleConnection(conn net.Conn) {
	br := bufio.NewReader(conn)
	for {
		// Set timeout
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			log.Printf("Failed to set timeout for connection %v", conn)
			_ = conn.Close()
			return
		}

		// Read next request from the client
		req, _, err := ReadRequest(br)

		// Handle EOF
		if errors.Is(err, io.EOF) {
			log.Printf("Connection closed by %v", conn.RemoteAddr())
			_ = conn.Close()
			return
		}

		// timeout in this application means we just close the connection
		if err, ok := err.(net.Error); ok && err.Timeout() {
			log.Printf("Connection to %v timed out", conn.RemoteAddr())
			_ = conn.Close()
			return
		}

		// Handle the request which is not a GET and immediately close the connection and return
		if err != nil {
			log.Printf("Handle bad request for error: %v", err)
			res := &Response{}
			res.HandleBadRequest()
			_ = res.Write(conn)
			_ = conn.Close()
			return
		}

		res := s.HandleGoodRequest(req)
		err = res.Write(conn)
		if err != nil {
			fmt.Println(err)
		}
	}
}

// HandleGoodRequest handles the valid req and generates the corresponding res.
func (s *Server) HandleGoodRequest(req *Request) (res *Response) {
	// Hint: use the other methods below
	res = &Response{}
	root := s.DocRoot
	url := req.URL
	l := len(url)
	if root == "" {
		root = "testdata/"
	}
	if url == "/" {
		url = "/index.html"
	} else if string(url[l-1]) == "/" {
		url += "index.html"
	}
	filePath := filepath.Join(root, url)
	filePath = filepath.Clean(filePath)

	url, err1 := filepath.Abs(filePath)
	if err1 != nil {
		res.HandleNotFound(req)
		return
	}
	req.URL = url

	directory, err2 := filepath.Abs(root)
	if err2 != nil {
		res.HandleNotFound(req)
		return
	}
	if !strings.HasPrefix(url, directory) || !fileExists(url) || isValidDir(url) {
		res.HandleNotFound(req)
		return
	}

	res.HandleOK(req, url)

	return res
}

// HandleOK prepares res to be a 200 OK response
// ready to be written back to client.
func (res *Response) HandleOK(req *Request, path string) {
	res.Proto = responseProto
	res.StatusCode = statusOK
	res.FilePath = path

	m := make(map[string]string)
	contentLength := getContentLength(path)
	m["Content-Length"] = contentLength
	m["Date"] = FormatTime(time.Now())
	m["Last-Modified"] = getLastModifiedTime(path)
	m["Content-Type"] = MIMETypeByExtension(filepath.Ext(path))
	if req.Close {
		m["Connection"] = "close"
	}
	res.Header = m

	if contentLength == "" {
		res.StatusCode = statusMethodNotFound
		res.FilePath = ""
	}
}

// HandleBadRequest prepares res to be a 400 Bad Request response
// ready to be written back to client.
func (res *Response) HandleBadRequest() {
	res.Proto = responseProto
	res.StatusCode = statusMethodNotAllowed

	m := make(map[string]string)
	m["Date"] = FormatTime(time.Now())
	m["Connection"] = "close"
	res.Header = m
}

// HandleNotFound prepares res to be a 404 Not Found response
// ready to be written back to client.
func (res *Response) HandleNotFound(req *Request) {
	res.Proto = responseProto
	res.StatusCode = statusMethodNotFound

	m := make(map[string]string)
	m["Date"] = FormatTime(time.Now())

	if req.Close {
		m["Connection"] = "close"
	}

	res.Header = m
}

//get last modified time of the file
func getLastModifiedTime(filename string) string {
	file, err := os.Stat(filename)
	if err != nil {
		return ""
	}
	mtime := file.ModTime()
	return FormatTime(mtime)
}

func getContentLength(filename string) string {
	fmt.Println("File  ", filename)
	file, err := os.Stat(filename)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(file.Size(), 10)
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func isValidDir(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return true
	}
	if fileInfo.IsDir() && string(path[len(path)-1]) != "/" {
		return true
	}
	return false
}
