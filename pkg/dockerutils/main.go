package dockerutils

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gopkg.in/zabawaba99/firego.v1"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	SECRETS_SRC            = "src/github.com/maddyonline/optcode-secrets/optimal-code-admin.json"
	DEFAULT_API_VERSION    = "1.26"
	DOCKER_TLS_VERIFY_KEY  = "DOCKER_TLS_VERIFY"
	DOCKER_HOST_KEY        = "DOCKER_HOST"
	DOCKER_CERT_PATH_KEY   = "DOCKER_CERT_PATH"
	DOCKER_API_VERSION_KEY = "DOCKER_API_VERSION"
)

var (
	dir         *ClientsDir = &ClientsDir{}
	firebaseDB  *firego.Firebase
	WorkingDir  string
	DOCKER_KEYS = []string{DOCKER_TLS_VERIFY_KEY, DOCKER_HOST_KEY, DOCKER_CERT_PATH_KEY}
	PEM_KEYS    = []string{"ca", "cert", "key"}
)

func workdir() (*string, error) {
	const WORKDIR = "dockerutils"
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	wd, err := filepath.Abs(filepath.Join(usr.HomeDir, WORKDIR))
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Join(wd, "cache"), 0700)
	if err != nil {
		return nil, err
	}
	return &wd, nil
}

func GetDir() *ClientsDir {
	return dir
}

func Init() error {
	wd, err := workdir()
	if err != nil {
		return err
	}
	firebaseDB, err = authenticatedFirebase()
	if err != nil {
		return err
	}
	WorkingDir = *wd
	return nil
}

type ClientType string

const (
	LocalEnv  ClientType = "LocalEnv"
	RemoteEnv            = "RemoteEnv"
)

type Entry struct {
	Client *client.Client
	Err    error
	Type   ClientType
	Name   string
}

func (cd Entry) String() string {
	return fmt.Sprintf("Client=%v, Error=%v, Type=%v, Name=%s", cd.Client, cd.Err, cd.Type, cd.Name)
}

type ClientsDir struct {
	Entries []Entry
}

func (d *ClientsDir) Add(cli *client.Client, err error, clitype ClientType, name string) {
	if d.Entries == nil {
		d.Entries = []Entry{}
	}
	d.Entries = append(d.Entries, Entry{cli, err, clitype, name})
}

func authenticatedFirebase() (*firego.Firebase, error) {
	gopath := os.Getenv("GOPATH")
	secretFile := filepath.Join(gopath, SECRETS_SRC)
	d, err := ioutil.ReadFile(secretFile)
	if err != nil {
		return nil, err
	}
	conf, err := google.JWTConfigFromJSON(d,
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/firebase.database")
	if err != nil {
		return nil, err
	}

	f := firego.New("https://optimal-code-admin.firebaseio.com", conf.Client(oauth2.NoContext))
	return f, nil
}

func readFromFirebase(f *firego.Firebase, name string) (map[string]string, error) {
	data := map[string]string{}
	if err := f.Child(name).Value(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func saveToFirebase(f *firego.Firebase, path string, data map[string]string) error {
	if err := f.Child(path).Set(data); err != nil {
		return err
	}
	return nil
}

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
	for _, key := range DOCKER_KEYS {
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

func getFilesForRelocation(envmap map[string]string) (map[string]string, error) {
	m := map[string]string{}
	for _, filename := range PEM_KEYS {
		f := filepath.Join(envmap[DOCKER_CERT_PATH_KEY], filename+".pem")
		data, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}
		m[filename] = string(data)
	}
	return m, nil
}

func relocate(name string, m map[string]string, updateDB bool) (*string, error) {
	dir := filepath.Join(WorkingDir, "docker_root", name, ".docker")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	for _, k := range PEM_KEYS {
		if err := ioutil.WriteFile(filepath.Join(dir, k+".pem"), []byte(m[k]), 0600); err != nil {
			return nil, err
		}
	}
	if updateDB {
		err := saveToFirebase(firebaseDB, name, m)
		if err != nil {
			return nil, err
		}
	}
	return &dir, nil
}

func RestoreEnvmapFromDB(name string) (map[string]string, error) {
	m, err := readFromFirebase(firebaseDB, name)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("Got empty map from DB")
	}
	path, err := relocate(name, m, false)
	if err != nil {
		return nil, err
	}
	envmap := map[string]string{}
	for _, k := range DOCKER_KEYS {
		envmap[k] = m[k]
	}
	envmap[DOCKER_CERT_PATH_KEY] = *path
	return envmap, nil
}

func RelocateEnvFile(name string, r io.Reader) (map[string]string, error) {
	envmap, err := ReadEnvFile(r)
	if err != nil {
		return nil, err
	}

	if err := verifyKeysPresent(envmap); err != nil {
		return nil, err
	}
	if err := verifyFilesAccessible(envmap); err != nil {
		return nil, err
	}
	m, err := getFilesForRelocation(envmap)
	if err != nil {
		return nil, err
	}
	for _, k := range DOCKER_KEYS {
		m[k] = envmap[k]
	}
	path, err := relocate(name, m, true)
	if err != nil {
		return nil, err
	}
	envmap[DOCKER_CERT_PATH_KEY] = *path
	return envmap, nil
}

func SaveEnvFile(name string, r io.Reader) error {
	content, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(WorkingDir, "cache", name), content, 0600)
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

func addEnvMapClient(r io.Reader, name string) error {
	envmap, err := ReadEnvFile(r)
	if err != nil {
		return err
	}
	cli, err := NewEnvMapClient(envmap)
	dir.Add(cli, err, RemoteEnv, name)
	return nil
}

func InitMachines(names []string) {
	Init()
	for _, name := range names {
		if name == "local" {
			cli, err := client.NewEnvClient()
			dir.Add(cli, err, LocalEnv, name)
			continue
		}
		if envmap, err := RestoreEnvmapFromDB(name); err == nil {
			cli, err := NewEnvMapClient(envmap)
			dir.Add(cli, err, RemoteEnv, name)
		}
	}
}

func addMachinesFromCache() error {
	dirname := filepath.Join(WorkingDir, "cache")
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".env") {
			continue
		}
		r, err := os.Open(filepath.Join(dirname, f.Name()))
		if err != nil {
			return err
		}
		defer r.Close()
		if err := addEnvMapClient(r, f.Name()); err != nil {
			return err
		}
	}
	return nil
}

func GetMachine() *client.Client {
	for _, entry := range dir.Entries {
		log.Printf("Trying %s %s", entry.Name, entry.Type)
		ping, err := entry.Client.Ping(context.Background())
		log.Printf("Got: ping=%v, err=%v", ping, err)
		if err == nil {
			return entry.Client
		}
	}
	return nil
}

func ListMachines() string {
	arr := []string{}
	for _, entry := range dir.Entries {
		ping, err := entry.Client.Ping(context.Background())
		arr = append(arr, fmt.Sprintf("%s| PING: (%v, %v)", entry, ping, err))
	}
	return strings.Join(arr, "\n")
}
