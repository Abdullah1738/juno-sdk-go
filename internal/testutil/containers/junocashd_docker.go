//go:build docker

package containers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	build "github.com/docker/docker/api/types/build"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultJunocashVersion = "0.9.13"
	defaultRPCUser         = "rpcuser"
	defaultRPCPassword     = "rpcpass"
)

type Junocashd struct {
	ContainerID string
	RPCURL      string
	RPCUser     string
	RPCPassword string

	c testcontainers.Container
}

func StartJunocashd(ctx context.Context) (*Junocashd, error) {
	version := defaultJunocashVersion
	rpcUser := defaultRPCUser
	rpcPass := defaultRPCPassword

	req := testcontainers.ContainerRequest{
		ImagePlatform: "linux/amd64",
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    repoRoot(),
			Dockerfile: "docker/junocashd/Dockerfile",
			BuildArgs: map[string]*string{
				"JUNOCASH_VERSION": &version,
			},
			BuildOptionsModifier: func(opts *build.ImageBuildOptions) {
				opts.Platform = "linux/amd64"
				opts.Version = build.BuilderBuildKit
			},
		},
		ExposedPorts: []string{"8232/tcp"},
		Cmd: []string{
			"-regtest",
			"-server=1",
			"-txindex=1",
			"-daemon=0",
			"-listen=0",
			"-printtoconsole=1",
			"-datadir=/data",
			"-rpcbind=0.0.0.0",
			"-rpcallowip=0.0.0.0/0",
			"-rpcport=8232",
			"-rpcuser=" + rpcUser,
			"-rpcpassword=" + rpcPass,
		},
		WaitingFor: wait.ForListeningPort(nat.Port("8232/tcp")).WithStartupTimeout(60 * time.Second),
	}
	if os.Getenv("JUNO_TEST_LOG") != "" {
		req.FromDockerfile.BuildLogWriter = os.Stdout
		req.LogConsumerCfg = &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		}
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, err
	}

	rpcPort, err := c.MappedPort(ctx, nat.Port("8232/tcp"))
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, err
	}

	return &Junocashd{
		ContainerID: c.GetContainerID(),
		RPCURL:      fmt.Sprintf("http://%s:%s", host, rpcPort.Port()),
		RPCUser:     rpcUser,
		RPCPassword: rpcPass,
		c:           c,
	}, nil
}

func (j *Junocashd) Terminate(ctx context.Context) error {
	if j == nil || j.c == nil {
		return nil
	}
	return j.c.Terminate(ctx)
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
