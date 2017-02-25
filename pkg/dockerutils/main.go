package dockerutils

import (
	"fmt"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DEFAULT_API_VERSION = "1.26"

const DOCKER_TLS_VERIFY_KEY = "DOCKER_TLS_VERIFY"
const DOCKER_HOST_KEY = "DOCKER_HOST"
const DOCKER_CERT_PATH_KEY = "DOCKER_CERT_PATH"
const DOCKER_API_VERSION_KEY = "DOCKER_API_VERSION"

type ClientType string

const (
	LocalEnv  ClientType = "LocalEnv"
	RemoteEnv            = "RemoteEnv"
)

type Entry struct {
	Client *client.Client
	Err    error
	Type   ClientType
}

func (cd Entry) String() string {
	return fmt.Sprintf("Client=%v, Error=%v, Type=%v", cd.Client, cd.Err, cd.Type)
}

type ClientsDir struct {
	Entries []Entry
}

func (d *ClientsDir) Add(cli *client.Client, err error, clitype ClientType) {
	if d.Entries == nil {
		d.Entries = []Entry{}
	}
	d.Entries = append(d.Entries, Entry{cli, err, clitype})
}

var dir *ClientsDir

func ReadEnvFile(r io.Reader) (map[string]string, error) {
	s, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(s), "\n")
	m := map[string]string{}
	for _, line := range lines {
		if len(line) > len("export") && line[:len("export")] == "export" {
			keyvals := strings.Split(line[len("export"):], "=")
			if len(keyvals) != 2 {
				continue
			}
			key := strings.Trim(keyvals[0], " ")
			val := strings.Trim(keyvals[1], " ")
			unquoted, err := strconv.Unquote(val)
			if err == nil {
				val = unquoted
			}
			m[key] = val
		}
	}

	return m, nil
}

func verifyKeysPresent(envmap map[string]string) error {
	KEYS := []string{DOCKER_TLS_VERIFY_KEY, DOCKER_HOST_KEY, DOCKER_CERT_PATH_KEY}
	for _, key := range KEYS {
		if _, ok := envmap[key]; !ok {
			return fmt.Errorf("Env map does not contain key %s", key)
		}
	}
	return nil
}

func verifyFilesAccessible(envmap map[string]string) error {
	for _, filename := range []string{"ca.pem", "cert.pem", "key.pem"} {
		f := filepath.Join(envmap[DOCKER_CERT_PATH_KEY], filename)
		if _, err := os.Stat(f); err != nil {
			return err
		}
	}
	return nil
}

func NewEnvMapClient(envmap map[string]string) (*client.Client, error) {
	if err := verifyKeysPresent(envmap); err != nil {
		return nil, err
	}
	if err := verifyFilesAccessible(envmap); err != nil {
		return nil, err
	}
	var httpClient *http.Client
	options := tlsconfig.Options{
		CAFile:             filepath.Join(envmap[DOCKER_CERT_PATH_KEY], "ca.pem"),
		CertFile:           filepath.Join(envmap[DOCKER_CERT_PATH_KEY], "cert.pem"),
		KeyFile:            filepath.Join(envmap[DOCKER_CERT_PATH_KEY], "key.pem"),
		InsecureSkipVerify: envmap[DOCKER_TLS_VERIFY_KEY] == "",
	}
	tlsc, err := tlsconfig.Client(options)
	if err != nil {
		panic(err)
	}
	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsc,
		},
	}
	apiVersion := envmap[DOCKER_API_VERSION_KEY]
	if apiVersion == "" {
		apiVersion = DEFAULT_API_VERSION
	}
	return client.NewClient(envmap[DOCKER_HOST_KEY], apiVersion, httpClient, nil)
}

func Init() {
	dir = &ClientsDir{}
	cli, err := client.NewEnvClient()
	dir.Add(cli, err, LocalEnv)
}

func AddEnvMapClient(r io.Reader) error {
	envmap, err := ReadEnvFile(r)
	if err != nil {
		return err
	}
	cli, err := NewEnvMapClient(envmap)
	dir.Add(cli, err, RemoteEnv)
	return nil
}

func ListMachines() string {
	arr := []string{}
	for _, entry := range dir.Entries {
		arr = append(arr, fmt.Sprintf("%s", entry))
	}
	return strings.Join(arr, "\n")
}
