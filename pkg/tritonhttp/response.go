package tritonhttp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
)

var statusText = map[int]string{
	statusOK:               "OK",
	statusMethodNotAllowed: "Bad Request",
	statusMethodNotFound:   "Not Found",
}

type Response struct {
	StatusCode int    // e.g. 200
	Proto      string // e.g. "HTTP/1.1"

	// Header stores all headers to write to the response.
	// Header keys are case-incensitive, and should be stored
	// in the canonical format in this map.
	Header map[string]string

	// Request is the valid request that leads to this response.
	// It could be nil for responses not resulting from a valid request.
	Request *Request

	// FilePath is the local path to the file to serve.
	// It could be "", which means there is no file to serve.
	FilePath string
}

// Write writes the res to the w.
func (res *Response) Write(w io.Writer) error {
	if err := res.WriteStatusLine(w); err != nil {
		return err
	}
	if err := res.WriteSortedHeaders(w); err != nil {
		return err
	}
	if err := res.WriteBody(w); err != nil {
		return err
	}
	return nil
}

// WriteStatusLine writes the status line of res to w, including the ending "\r\n".
// For example, it could write "HTTP/1.1 200 OK\r\n".
func (res *Response) WriteStatusLine(w io.Writer) error {
	bw := bufio.NewWriter(w)

	statusLine := fmt.Sprintf("%v %v %v\r\n", res.Proto, res.StatusCode, statusText[res.StatusCode])
	if _, err := bw.WriteString(statusLine); err != nil {
		return err
	}

	if err := bw.Flush(); err != nil {
		return err
	}
	return nil
}

// WriteSortedHeaders writes the headers of res to w, including the ending "\r\n".
// For example, it could write "Connection: close\r\nDate: foobar\r\n\r\n".
// For HTTP, there is no need to write headers in any particular order.
// TritonHTTP requires to write in sorted order for the ease of testing.
func (res *Response) WriteSortedHeaders(w io.Writer) error {
	response := ""
	delimiter := "\r\n"
	responseMap := make(map[string]string)
	keys := make([]string, 0, len(responseMap))
	for k, v := range res.Header {
		keys = append(keys, k)
		responseMap[k] = v
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := m[k]
		fmt.Println(k, v)
		line := k + ": " + v
		response = response + line + delimiter
	}

	response = response + delimiter
	bw := bufio.NewWriter(w)
	if _, err := bw.WriteString(response); err != nil {
		return err
	}

	if err := bw.Flush(); err != nil {
		return err
	}
	return nil
}

// WriteBody writes res' file content as the response body to w.
// It doesn't write anything if there is no file to serve.
func (res *Response) WriteBody(w io.Writer) error {
	if res.FilePath == "" {
		//Nothing to write, returning
		return nil
	}

	bw := bufio.NewWriter(w)

	var BufferSize int64 = 100
	file, err := os.Open(res.FilePath)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fi, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return err
	}

	filesize := fi.Size()
	defer file.Close()

	buffer := make([]byte, BufferSize)

	var i int64 = 0

	for i = 0; i < filesize/BufferSize; i++ {
		_, err := file.Read(buffer)

		if err != nil {
			if err != io.EOF {
				fmt.Println(err)
			}
			break
		}

		if _, err := bw.Write(buffer); err != nil {
			return err
		}
		if err := bw.Flush(); err != nil {
			return err
		}
	}
	buffer = make([]byte, filesize%BufferSize)
	_, err = file.Read(buffer)
	if err != nil {
		if err != io.EOF {
			fmt.Println(err)
		}
	}
	if _, err := bw.Write(buffer); err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}

	bw.Flush()
	return nil
}
