package netconf

import (
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

type Client interface {
	Close() error
	DeleteConfig(applyGroup string, commit bool) (string, error)
	SendCommit() error
	MarshalGroup(id string, obj interface{}) error
	SendTransaction(id string, obj interface{}, commit bool) error
}

func publicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}
