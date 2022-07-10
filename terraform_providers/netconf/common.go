package netconf

import (
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

type Client interface {
	Close() error
	DeleteConfig(applyGroup string, commit bool) (string, error)
	SendCommit(check bool) error
	MarshalGroup(id string, obj interface{}) error
	SendTransaction(id string, obj interface{}, commit bool) error
}

const validateCandidate = `<validate> 
<source> 
	<candidate/> 
</source> 
</validate>`

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
