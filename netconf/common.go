package netconf

type Client interface {
	Close() error
	ReadGroup(applygroup string) (string, error)
	UpdateRawConfig(applygroup string, netconfcall string, commit bool) (string, error)
	DeleteConfig(applygroup string) (string, error)
	DeleteConfigNoCommit(applygroup string) (string, error)
	SendCommit() error
	MarshalGroup(id string, obj interface{}) error
	SendTransaction(id string, obj interface{}, commit bool) error
	SendRawConfig(netconfcall string, commit bool) (string, error)
	ReadRawGroup(applygroup string) (string, error)
}

//func NewClient(username string, password string, sshkey string, address string, port int) (*GoNCClient, error) {
//	s, err := NewNetconfClient(username, password, sshkey, address, port)
//	if err != nil {
//		return nil, err
//	}
//	out := s.(*GoNCClient)
//	return out, nil
//}
