package hetzner

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/codegangsta/cli"
	"github.com/docker/machine/drivers"
	"github.com/docker/machine/log"
	"github.com/docker/machine/ssh"
	"github.com/docker/machine/state"
	"encoding/json"
)

type Driver struct {
	*drivers.BaseDriver
	Login    string
	Password string
}

func init() {
	drivers.Register("hetzner", &drivers.RegisteredDriver{
		New:            NewDriver,
		GetCreateFlags: GetCreateFlags,
	})
}

func GetCreateFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name: "hetzner-ip-address",
			Usage: "IP Address of machine",
			Value: "",
		},
		cli.StringFlag{
			Name: "hetzner-login",
			Usage: "Login for Hetzner Robot",
			Value: "",
		},
		cli.StringFlag{
			Name: "hetzner-password",
			Usage: "Password for Hetzner Robot",
			Value: "",
		},
	}
}

func NewDriver(machineName string, storePath string, caCert string, privateKey string) (drivers.Driver, error) {
	inner := drivers.NewBaseDriver(machineName, storePath, caCert, privateKey)
	return &Driver{BaseDriver: inner}, nil
}

func (d *Driver) Create() error {
	fingerprint, err := d.createKeyPair()
	if err != nil {
		return fmt.Errorf("unable to create key pair: %s", err)
	}

	// http://wiki.hetzner.de/index.php/Robot_Webservice/en#POST_.2Fboot.2F.3Cserver-ip.3E.2Flinux
	linux := fmt.Sprintf("/boot/%s/linux", d.IPAddress)
	resp, err := d.robotApiCall(linux, url.Values{
		"dist": {"Ubuntu 14.04.2 LTS minimal"},
		"arch": {"64"},
		"lang": {"en"},
		"authorized_key": {fingerprint},
	})

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// http://wiki.hetzner.de/index.php/Robot_Webservice/en#POST_.2Freset.2F.3Cserver-ip.3E
	reset := fmt.Sprintf("/reset/%s", d.IPAddress)
	resp, err = d.robotApiCall(reset,	url.Values{"type": {"hw"}})
	defer resp.Body.Close()

	return err
}

func (d *Driver) GetIP() (string, error) {
	if d.IPAddress == "" {
		return "", fmt.Errorf("IP address is not set")
	}
	return d.IPAddress, nil
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	if ip == "" {
		return "", nil
	}
	return fmt.Sprintf("tcp://%s:2376", ip), nil
}

func (d *Driver) GetState() (state.State, error) {
	return state.Running, nil
}

func (d *Driver) Kill() error {
	return fmt.Errorf("not yet implemented")
}

func (d *Driver) PreCreateCheck() error {
	return nil
}

func (d *Driver) Remove() error {
	return fmt.Errorf("not yet implemented")
}

func (d *Driver) Restart() error {
	return fmt.Errorf("not yet implemented")
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	d.IPAddress = flags.String("hetzner-ip-address")
	d.Login = flags.String("hetzner-login")
	d.Password = flags.String("hetzner-password")

	if d.IPAddress == "" {
		return fmt.Errorf("hetzner driver requires the --hetzner-ip-address option")
	}

	if d.Login == "" {
		return fmt.Errorf("hetzner driver requires the --hetzner-login option")
	}

	if d.Password == "" {
		return fmt.Errorf("hetzner driver requires the --hetzner-password option")
	}

	return nil
}

func (d *Driver) Start() error {
	return fmt.Errorf("not yet implemented")
}

func (d *Driver) Stop() error {
	return fmt.Errorf("not yet implemented")
}


func (d *Driver) createKeyPair() (string, error) {

	if err := ssh.GenerateSSHKey(d.GetSSHKeyPath()); err != nil {
		return "", err
	}

	publicKey, err := ioutil.ReadFile(d.GetSSHKeyPath() + ".pub")
	if err != nil {
		return "", err
	}

	keyName := d.MachineName

	log.Debugf("creating key pair: %s", keyName)

	return d.importKeyPair(keyName, string(publicKey))
}

type KeyRespone struct {
	Key Key `json:"key"`
}

type Key struct {
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	Type        string `json:"type"`
	Size        int    `json:"size"`
	Data        string `json:"data"`
}

// http://wiki.hetzner.de/index.php/Robot_Webservice/en#POST_.2Fkey
func (d *Driver) importKeyPair(name, publicKey string) (string, error) {
	resp, err := d.robotApiCall("/key", url.Values{
		"name": {name},
		"data": {publicKey},
	})

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	unmarshalledResponse := KeyRespone{}
	err = json.Unmarshal(contents, &unmarshalledResponse)
	return unmarshalledResponse.Key.Fingerprint, err
}

// http://wiki.hetzner.de/index.php/Robot_Webservice/en
func (d *Driver) robotApiCall(path string, v url.Values) (*http.Response, error) {
	url := fmt.Sprintf("https://%s:%s@robot-ws.your-server.de%s",
		d.Login,
		d.Password,
		path,
	)
	resp, err := http.PostForm(url, v)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return resp, fmt.Errorf("%s", resp)
	}
	return resp, nil
}