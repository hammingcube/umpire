package dockerutils

import (
	"strings"
	"testing"
)

const EXAMPLE_CONFIG = `
export DOCKER_TLS_VERIFY="1"
export DOCKER_HOST="tcp://54.149.203.125:2376"
export DOCKER_CERT_PATH="/Users/maddy/.docker/machine/machines/awsdocker"
export DOCKER_MACHINE_NAME="awsdocker"
# Run this command to configure your shell:
# eval $(docker-machine env awsdocker)
`

func TestInitMachines(t *testing.T) {
	Init()
	err := InitMachines()
	if err != nil {
		t.Error(err)
	}
	t.Logf("machines: %s", ListMachines())
}

func TestWorkingDir(t *testing.T) {
	s, err := workdir()
	if s == nil || err != nil {
		t.Error(err)
	}
	t.Logf("Got wd=%s", *s)
}

func TestSaveEnvFile(t *testing.T) {
	Init()
	if err := SaveEnvFile("my-remote-docker.env", strings.NewReader(EXAMPLE_CONFIG)); err != nil {
		t.Error(err)
	}
}

func TestRelocateEnvFile(t *testing.T) {
	Init()
	envmap, err := RelocateEnvFile("my-remote-docker.env", strings.NewReader(EXAMPLE_CONFIG))
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v", envmap)
	cli, err := NewEnvMapClient(envmap)
	dir.Add(cli, err, RemoteEnv)
	t.Logf(ListMachines())
}

func TestReadEnvFile(t *testing.T) {
	r := strings.NewReader(EXAMPLE_CONFIG)
	m, err := ReadEnvFile(r)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if err := verifyKeysPresent(m); err != nil {
		t.Errorf("Error: %v", err)
	}
	t.Logf("map: %v", m)
}

func TestClients(t *testing.T) {
	Init()
	t.Logf("machines: %s", ListMachines())
	r := strings.NewReader(EXAMPLE_CONFIG)
	if err := AddEnvMapClient(r); err != nil {
		t.Error(err)
	}
	t.Logf("machines: %s", ListMachines())
}
