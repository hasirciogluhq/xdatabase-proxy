// Package protocol reads the login of database protocols that send credentials
// in their first client message, so the proxy can route by username.
package protocol

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

const (
	maxRedisArgs  = 32
	maxRedisBytes = 4096
)

// Redis reads the first command, which must carry credentials:
// AUTH <user> <password>  or  HELLO <version> AUTH <user> <password> [...].
// Clients that send something else first (HELLO without AUTH, PING, or a
// password-only AUTH) cannot be routed by username.
type Redis struct{}

func (Redis) Read(r *bufio.Reader) (string, func(string) []byte, error) {
	args, err := readRESP(r)
	if err != nil {
		return "", nil, err
	}
	idx := 0
	switch strings.ToUpper(args[0]) {
	case "AUTH":
		if len(args) == 3 {
			idx = 1
		}
	case "HELLO":
		for i := 2; i+2 < len(args); i++ {
			if strings.EqualFold(args[i], "AUTH") {
				idx = i + 1
				break
			}
		}
	}
	if idx == 0 {
		return "", nil, errors.New("first command must be AUTH <user> <password> or HELLO <version> AUTH <user> <password>")
	}
	forward := func(plain string) []byte {
		a := slices.Clone(args)
		a[idx] = plain
		return encodeRESP(a)
	}
	return args[idx], forward, nil
}

// WriteError sends a RESP error to the client.
func (Redis) WriteError(w io.Writer, msg string) {
	fmt.Fprintf(w, "-ERR %s\r\n", strings.NewReplacer("\r", " ", "\n", " ").Replace(msg))
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadSlice('\n')
	if err != nil {
		return "", err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return "", errors.New("malformed RESP line")
	}
	return string(line[:len(line)-2]), nil
}

// readRESP reads one command encoded as an array of bulk strings.
func readRESP(r *bufio.Reader) ([]string, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	if line == "" || line[0] != '*' {
		return nil, errors.New("inline commands are not supported")
	}
	n, err := strconv.Atoi(line[1:])
	if err != nil || n < 1 || n > maxRedisArgs {
		return nil, errors.New("invalid RESP array length")
	}
	args, total := make([]string, n), 0
	for i := range args {
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		if line == "" || line[0] != '$' {
			return nil, errors.New("expected RESP bulk string")
		}
		l, err := strconv.Atoi(line[1:])
		if err != nil || l < 0 || l > maxRedisBytes {
			return nil, errors.New("invalid RESP bulk length")
		}
		if total += l; total > maxRedisBytes {
			return nil, errors.New("command too large")
		}
		buf := make([]byte, l+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		if buf[l] != '\r' || buf[l+1] != '\n' {
			return nil, errors.New("malformed RESP bulk string")
		}
		args[i] = string(buf[:l])
	}
	return args, nil
}

func encodeRESP(args []string) []byte {
	b := []byte("*" + strconv.Itoa(len(args)) + "\r\n")
	for _, a := range args {
		b = append(b, "$"+strconv.Itoa(len(a))+"\r\n"...)
		b = append(append(b, a...), '\r', '\n')
	}
	return b
}
