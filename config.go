package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// account is one saved connection. The secret key never goes into this
// file: it lives in the system's credential store, under the account ID.
type account struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	AK       string `json:"ak"`
	Endpoint string `json:"endpoint"`
	Path     string `json:"path,omitempty"`
}

// Label names the account in the connection list: its name, or the masked
// access key when it has none.
func (a account) Label() string {
	if a.Name != "" {
		return a.Name
	}
	if len(a.AK) > 10 {
		return a.AK[:4] + "…" + a.AK[len(a.AK)-4:]
	}
	return a.AK
}

type config struct {
	Accounts    []account `json:"accounts,omitempty"`
	LastID      string    `json:"lastId,omitempty"`
	DownloadDir string    `json:"downloadDir,omitempty"`
	Favorites   []string  `json:"favorites,omitempty"`
}

func (c *config) upsert(a account) {
	for i := range c.Accounts {
		if c.Accounts[i].ID == a.ID {
			c.Accounts[i] = a
			c.LastID = a.ID
			return
		}
	}
	c.Accounts = append(c.Accounts, a)
	c.LastID = a.ID
}

func (c *config) last() *account {
	for i := range c.Accounts {
		if c.Accounts[i].ID == c.LastID {
			return &c.Accounts[i]
		}
	}
	if len(c.Accounts) > 0 {
		return &c.Accounts[0]
	}
	return nil
}

// accountID names a connection by what identifies it, so that connecting
// again with the same key and endpoint updates it rather than adding another.
func accountID(ak, endpoint string) string {
	sum := sha256.Sum256([]byte(ak + "\x00" + endpoint))
	return hex.EncodeToString(sum[:8])
}

func configFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "OBS Go", "config.json"), nil
}

func loadConfig() config {
	var c config
	path, err := configFile()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	return c
}

func saveConfig(c config) error {
	path, err := configFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var secrets = mygo.Secrets.Service("com.obsgo.desktop")

// keychainEnabled lets a test or a locked-down deployment keep secrets in
// memory only.
func keychainEnabled() bool {
	return os.Getenv("OBSGO_NO_KEYCHAIN") == ""
}

func saveSecret(id, sk string) error {
	return secrets.Set("sk:"+id, []byte(sk))
}

func loadSecret(id string) (string, error) {
	b, err := secrets.Get("sk:" + id)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
