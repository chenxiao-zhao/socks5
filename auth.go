package socks5

import (
	"bytes"
	"fmt"
	"hash"
	"io"
	"sync"
)

type Authenticator interface {
	Authenticate(in io.Reader, out io.Writer) error
}

// NoAuth NO_AUTHENTICATION_REQUIRED
type NoAuth struct {
}

// Authenticate NO_AUTHENTICATION_REQUIRED Authentication for SOCKS V5
func (n NoAuth) Authenticate(in io.Reader, out io.Writer) error {
	//send reply to client,format is as follows:
	//         +----+--------+
	//         |VER | METHOD |
	//         +----+--------+
	//         | 1  |   1    |
	//         +----+--------+
	reply := []byte{Version5, NO_AUTHENTICATION_REQUIRED}
	_, err := out.Write(reply)
	if err != nil {
		return err
	}

	return nil
}

type UserPwdAuth struct {
	UserPwdStore
}

// Authenticate Username/Password Authentication for SOCKS V5
func (u UserPwdAuth) Authenticate(in io.Reader, out io.Writer) error {
	uname, passwd, err := u.ReadUserPwd(in)
	if err != nil {
		return err
	}

	err = u.Validate(string(uname), string(passwd))
	if err != nil {
		reply := []byte{Version5, 1}
		_, err1 := out.Write(reply)
		if err1 != nil {
			return err
		}
		return err
	}

	//authentication successful,then send reply to client
	reply := []byte{Version5, 0}
	_, err = out.Write(reply)
	if err != nil {
		return err
	}

	return nil
}

// ReadUserPwd read Username/Password request from client
// return username and password, when
// Username/Password request format is as follows:
//    +----+------+----------+------+----------+
//    |VER | ULEN |  UNAME   | PLEN |  PASSWD  |
//    +----+------+----------+------+----------+
//    | 1  |  1   | 1 to 255 |  1   | 1 to 255 |
//    +----+------+----------+------+----------+
// For standard details, please see (https://www.rfc-editor.org/rfc/rfc1929.html)
func (u UserPwdAuth) ReadUserPwd(in io.Reader) ([]byte, []byte, error) {

	ulen := make([]byte, 2)
	_, err := io.ReadAtLeast(in, ulen, len(ulen))
	if err != nil {
		return nil, nil, err
	}

	uname := make([]byte, ulen[1])
	_, err = io.ReadAtLeast(in, uname, int(ulen[1]))
	if err != nil {
		return nil, nil, err
	}

	plen := make([]byte, 1)
	_, err = io.ReadAtLeast(in, plen, 1)
	if err != nil {
		return nil, nil, err
	}

	passwd := make([]byte, plen[0])
	_, err = io.ReadAtLeast(in, passwd, int(plen[0]))
	if err != nil {
		return nil, nil, err
	}

	return uname, passwd, nil
}

// UserPwdStore provide username and password storage
type UserPwdStore interface {
	Set(username string, password string) error
	Del(username string) error
	Validate(username string, password string) error
}

type MemoryStore struct {
	Users map[string][]byte
	mu    sync.Mutex
	hash.Hash
	algoSecret string
}

// Set the mapping of username and password
func (m *MemoryStore) Set(username string, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	build := bytes.NewBuffer(nil)
	build.WriteString(password + m.algoSecret)
	cryptPasswd := m.Hash.Sum(build.Bytes())
	m.Users[username] = cryptPasswd
	return nil
}

// UserNotExist the error type used in UserPwdStore.Del() method and
// UserPwdStore.Validate method
type UserNotExist struct {
	username string
}

func (u UserNotExist) Error() string {
	return fmt.Sprintf("user %s don't exist", u.username)
}

// Del delete by username
func (m *MemoryStore) Del(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Users[username]; !ok {
		return UserNotExist{username: username}
	}

	delete(m.Users, username)
	return nil
}

// Validate validate username and password
func (m *MemoryStore) Validate(username string, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Users[username]; !ok {
		return UserNotExist{username: username}
	}

	build := bytes.NewBuffer(nil)
	build.WriteString(password + m.algoSecret)
	cryptPasswd := m.Hash.Sum(build.Bytes())
	if !bytes.Equal(cryptPasswd, m.Users[username]) {
		return fmt.Errorf("user %s has bad password", username)
	}
	return nil
}
