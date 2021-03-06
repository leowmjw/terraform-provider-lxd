package lxd

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"

	"github.com/lxc/lxd"
	"github.com/lxc/lxd/shared"
)

type LxdProvider struct {
	Remote string
	Client *lxd.Client
}

// Provider returns a terraform.ResourceProvider.
func Provider() terraform.ResourceProvider {

	// The actual provider
	return &schema.Provider{
		Schema: map[string]*schema.Schema{

			"address": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["lxd_address"],
				DefaultFunc: schema.EnvDefaultFunc("LXD_ADDR", "/var/lib/lxd/unix.socket"),
			},

			"scheme": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["lxd_scheme"],
				DefaultFunc: schema.EnvDefaultFunc("LXD_SCHEME", "unix"),
			},

			"port": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["lxd_port"],
				DefaultFunc: schema.EnvDefaultFunc("LXD_PORT", "8443"),
			},

			"remote": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: descriptions["lxd_remote"],
				DefaultFunc: schema.EnvDefaultFunc("LXD_REMOTE", "local"),
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"lxd_container": resourceLxdContainer(),
			"lxd_network":   resourceLxdNetwork(),
			"lxd_profile":   resourceLxdProfile(),
		},

		ConfigureFunc: providerConfigure,
	}
}

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"lxd_address": "The FQDN or IP where the LXD daemon can be contacted. (default = /var/lib/lxd/unix.socket)",
		"lxd_scheme":  "unix or https (default = unix)",
		"lxd_port":    "Port LXD Daemon is listening on (default 8443).",
		"lxd_remote":  "Name of the LXD remote. Required when lxd_scheme set to https, to enable locating server certificate.",
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	remote := d.Get("remote").(string)
	scheme := d.Get("scheme").(string)

	daemon_addr := ""
	switch scheme {
	case "unix":
		daemon_addr = fmt.Sprintf("unix:%s", d.Get("address"))
	case "https":
		daemon_addr = fmt.Sprintf("https://%s:%s", d.Get("address"), d.Get("port"))
	default:
		err := fmt.Errorf("Invalid scheme: %s", scheme)
		return nil, err
	}

	// build LXD config
	config := lxd.Config{
		ConfigDir: os.ExpandEnv("$HOME/.config/lxc"),
		Remotes:   make(map[string]lxd.RemoteConfig),
	}
	config.Remotes[remote] = lxd.RemoteConfig{Addr: daemon_addr}
	log.Printf("[DEBUG] LXD Config: %#v", config)

	if scheme == "https" {
		// validate certifictes exist
		certf := config.ConfigPath("client.crt")
		keyf := config.ConfigPath("client.key")
		if !shared.PathExists(certf) || !shared.PathExists(keyf) {
			err := fmt.Errorf("Certificate or key not found:\n\t%s\n\t%s", certf, keyf)
			return nil, err
		}
		serverCertf := config.ServerCertPath(remote)
		if !shared.PathExists(serverCertf) {
			err := fmt.Errorf("Server certificate not found:\n\t%s", serverCertf)
			return nil, err
		}
	}

	client, err := lxd.NewClient(&config, remote)
	if err != nil {
		err := fmt.Errorf("Could not create LXD client: %s", err)
		return nil, err
	}
	log.Printf("[DEBUG] LXD Client: %#v", client)

	if err := validateClient(client); err != nil {
		return nil, err
	}

	lxdProv := LxdProvider{
		Remote: remote,
		Client: client,
	}

	return &lxdProv, nil
}

func validateClient(client *lxd.Client) error {
	if _, err := client.GetServerConfig(); err != nil {
		return err
	}
	return nil
}
