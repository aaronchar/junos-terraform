package netconf

import (
	"context"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

type Client interface {
	Close() error
	DeleteConfig(ctx context.Context, applyGroup string, commit bool) (string, error)
	SendCommit(ctx context.Context, check bool) error
	MarshalGroup(ctx context.Context, id string, obj interface{}) error
	SendTransaction(ctx context.Context, id string, obj interface{}, commit bool) error
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
