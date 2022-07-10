package netconf

import (
	"encoding/xml"
	"fmt"
	driver "github.com/davedotdev/go-netconf/drivers/driver"
	sshdriver "github.com/davedotdev/go-netconf/drivers/ssh"
	"golang.org/x/crypto/ssh"
	"strings"
	"sync"
)

const bulkGroupStrXML = `<load-configuration action="merge" format="xml">
<configuration>
%s
</configuration>
</load-configuration>
`

const bulkGetGroupXMLStr = `<get-configuration>
  <configuration>
  <groups></groups>
  </configuration>
</get-configuration>
`
const bulkValidateCandidate = `<validate> 
<source> 
	<candidate/> 
</source> 
</validate>`
const bulkReadWrapper = `<configuration>%s</configuration>`

const bulkCommitStr = `<commit/>`

const bulkDeletePayload = `<groups operation="delete">
	<name>%[1]s</name>
</groups>
<apply-groups operation="delete">%[1]s</apply-groups>`

const bulkDeleteStr = `<edit-config>
	<target>
		<candidate/>
	</target>
	<default-operation>none</default-operation> 
	<config>
		<configuration>
			%s
		</configuration>
	</config>
</edit-config>`

// BulkGoNCClient type for storing data and wrapping functions
type BulkGoNCClient struct {
	Driver driver.Driver
	Lock   sync.RWMutex
	BH     BatchHelper
}

// Close is a functional thing to close the Driver
func (g *BulkGoNCClient) Close() error {
	g.Driver = nil
	return nil
}

// updateRawConfig deletes group data and replaces it (for Update in TF)
func (g *BulkGoNCClient) updateRawConfig(applyGroup string, netconfCall string, _ bool) (string, error) {

	if err := g.BH.AddToDeleteMap(applyGroup); err != nil {
		return "", err
	}
	if err := g.BH.AddToWriteMap(netconfCall); err != nil {
		return "", err
	}

	return "", nil
}

// DeleteConfig is a wrapper for driver.SendRaw()
func (g *BulkGoNCClient) DeleteConfig(applyGroup string, _ bool) (string, error) {
	if err := g.BH.AddToDeleteMap(applyGroup); err != nil {
		return "", err
	}
	return "", nil
}

// SendCommit is a wrapper for driver.SendRaw()
func (g *BulkGoNCClient) SendCommit() error {

	g.Lock.Lock()
	defer g.Lock.Unlock()

	deleteCache := g.BH.QueryAllGroupDeletes()
	writeCache := g.BH.QueryAllGroupWrites()
	if err := g.Driver.Dial(); err != nil {
		return err
	}

	if deleteCache != "" {
		bulkDeleteString := fmt.Sprintf(bulkDeleteStr, deleteCache)
		// So on the commit we are going to send our entire delete-cache, if we get any load error
		// we return the full xml error response and exit
		bulkDeleteReply, err := g.Driver.SendRaw(bulkDeleteString)
		if err != nil {
			errInternal := g.Driver.Close()
			return fmt.Errorf("driver error: %+v, driver close error: %s", err, errInternal)
		}
		// I am doing string checks simply because it is most likely more efficient
		// than loading in through a xml parser
		if strings.Contains(bulkDeleteReply.Data, "operation-failed") {
			return fmt.Errorf("failed to write bulk delete %s", bulkDeleteReply.Data)
		}
	}
	if writeCache != "" {

		bulkCreateString := fmt.Sprintf(bulkGroupStrXML, writeCache)
		// So on the commit we are going to send our entire write-cache, if we get any load error
		// we return the full xml error response and exit
		bulkWriteReply, err := g.Driver.SendRaw(bulkCreateString)
		if err != nil {
			errInternal := g.Driver.Close()
			return fmt.Errorf("driver error: %+v, driver close error: %s", err, errInternal)
		}
		// I am doing string checks simply because it is most likely more efficient
		// than loading in through a xml parser
		if strings.Contains(bulkWriteReply.Data, "operation-failed") {
			return fmt.Errorf("failed to write bulk configuration %s", bulkWriteReply.Data)
		}
	}
	// we have loaded the full configuration without any error
	// before we can commit this we are going to do a commit check
	// if it fails we return the full xml error
	commitCheckReply, err := g.Driver.SendRaw(bulkValidateCandidate)
	if err != nil {
		errInternal := g.Driver.Close()
		return fmt.Errorf("driver error: %+v, driver close error: %s", err, errInternal)
	}

	// I am doing string checks simply because it is most likely more efficient
	// than loading in through a xml parser
	if !strings.Contains(commitCheckReply.Data, "commit-check-success") {
		return fmt.Errorf("candidate commit check failed %s", commitCheckReply.Data)
	}
	if _, err = g.Driver.SendRaw(bulkCommitStr); err != nil {
		return err
	}
	return nil
}

// MarshalGroup accepts a struct of type X and then marshals data onto it
func (g *BulkGoNCClient) MarshalGroup(id string, obj interface{}) error {
	reply, err := g.readRawGroup(id)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal([]byte(reply), &obj); err != nil {
		return err
	}
	return nil
}

// SendTransaction is a method that unnmarshals the XML, creates the transaction and passes in a commit
func (g *BulkGoNCClient) SendTransaction(id string, obj interface{}, commit bool) error {
	cfg, err := xml.Marshal(obj)
	if err != nil {
		return err
	}
	// updateRawConfig deletes old group by, re-creates it then commits.
	// As far as Junos cares, it's an edit.
	if id != "" {
		if _, err = g.updateRawConfig(id, string(cfg), commit); err != nil {
			return err
		}
		return nil
	}
	if _, err = g.sendRawConfig(string(cfg), commit); err != nil {
		return err
	}
	return nil
}

// sendRawConfig is a wrapper for driver.SendRaw()
func (g *BulkGoNCClient) sendRawConfig(netconfCall string, _ bool) (string, error) {
	if err := g.BH.AddToWriteMap(netconfCall); err != nil {
		return "", err
	}
	return "", nil
}

// readRawGroup is a helper function
func (g *BulkGoNCClient) readRawGroup(applyGroup string) (string, error) {
	// we are filling up the read buffer, this will only be done once regardless of the amount of \
	g.Lock.Lock()
	defer g.Lock.Unlock()

	if !g.BH.IsHydrated() {
		if err := g.hydrateReadCache(); err != nil {
			return "", err
		}
	}
	return g.BH.QueryGroupXMLFromCache(applyGroup)
}

// This is called on instantiation of the batch client, it is used to hydrate
// the read cache.
func (g *BulkGoNCClient) hydrateReadCache() error {
	if err := g.Driver.Dial(); err != nil {
		errInternal := g.Driver.Close()
		return fmt.Errorf("driver error: %+v, driver close error: %s", err, errInternal)
	}
	reply, err := g.Driver.SendRaw(bulkGetGroupXMLStr)
	if err != nil {
		errInternal := g.Driver.Close()
		return fmt.Errorf("driver error: %+v, driver close error: %s", err, errInternal)
	}
	if err := g.Driver.Close(); err != nil {
		return err
	}
	if err := g.BH.AddToReadMap(reply.Data); err != nil {
		return err
	}
	return nil
}

// NewBulkClient returns go-netconf new client driver
func NewBulkClient(username string, password string, sshkey string, address string, port int) (Client, error) {
	// Dummy interface var ready for loading from inputs
	var nconf driver.Driver

	d := driver.New(sshdriver.New())

	nc := d.(*sshdriver.DriverSSH)

	nc.Host = address
	nc.Port = port

	// SSH keys takes priority over password based
	if sshkey != "" {
		nc.SSHConfig = &ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				publicKeyFile(sshkey),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		// Sort yourself out with SSH. Easiest to do that here.
		nc.SSHConfig = &ssh.ClientConfig{
			User:            username,
			Auth:            []ssh.AuthMethod{ssh.Password(password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}
	nconf = nc
	c := &BulkGoNCClient{
		Driver: nconf,
		BH:     NewBatchHelper(),
	}
	return c, nil
}
