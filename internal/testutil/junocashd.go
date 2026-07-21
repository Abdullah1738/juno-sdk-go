package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Abdullah1738/juno-sdk-go/junocashd"
)

var ErrJunocashdNotFound = errors.New("junocashd not found")

type JunocashdConfig struct {
	JunocashdPath string
	JunocashCLI   string
	RPCUser       string
	RPCPassword   string
	TxIndex       bool
}

type RunningJunocashd struct {
	cmd         *exec.Cmd
	Datadir     string
	RPCURL      string
	RPCPort     int
	RPCUser     string
	RPCPassword string

	cliPath string

	dockerContainer string
	dockerDatadir   string
	dockerRPCPort   int

	stopOnce sync.Once
}

func StartJunocashd(ctx context.Context, cfg JunocashdConfig) (*RunningJunocashd, error) {
	if rpcURL := os.Getenv("JUNO_TEST_RPC_URL"); strings.TrimSpace(rpcURL) != "" {
		return connectExternalJunocashd(ctx, rpcURL)
	}

	bin := cfg.JunocashdPath
	if bin == "" {
		p, err := exec.LookPath("junocashd")
		if err != nil {
			return nil, ErrJunocashdNotFound
		}
		bin = p
	}

	rpcUser := cfg.RPCUser
	if rpcUser == "" {
		rpcUser = "rpcuser"
	}
	rpcPass := cfg.RPCPassword
	if rpcPass == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("rand: %w", err)
		}
		rpcPass = hex.EncodeToString(b)
	}

	rpcPort, err := freePort()
	if err != nil {
		return nil, err
	}
	p2pPort, err := freePort()
	if err != nil {
		return nil, err
	}

	datadir, err := os.MkdirTemp("", "junocashd-regtest-*")
	if err != nil {
		return nil, fmt.Errorf("mkdtemp: %w", err)
	}

	args := []string{
		"-regtest",
		"-server=1",
		"-daemon=0",
		"-listen=0",
		"-printtoconsole=1",
		fmt.Sprintf("-datadir=%s", datadir),
		fmt.Sprintf("-rpcbind=%s", "127.0.0.1"),
		fmt.Sprintf("-rpcallowip=%s", "127.0.0.1"),
		fmt.Sprintf("-rpcport=%d", rpcPort),
		fmt.Sprintf("-rpcuser=%s", rpcUser),
		fmt.Sprintf("-rpcpassword=%s", rpcPass),
		fmt.Sprintf("-port=%d", p2pPort),
	}
	if cfg.TxIndex {
		args = append(args, "-txindex=1")
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = datadir
	logs := io.Discard
	if os.Getenv("JUNO_TEST_LOG") != "" {
		logs = os.Stdout
	}
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(datadir)
		return nil, fmt.Errorf("start junocashd: %w", err)
	}

	r := &RunningJunocashd{
		cmd:         cmd,
		Datadir:     datadir,
		RPCURL:      fmt.Sprintf("http://127.0.0.1:%d", rpcPort),
		RPCPort:     rpcPort,
		RPCUser:     rpcUser,
		RPCPassword: rpcPass,
		cliPath:     defaultCLIPath(cfg.JunocashCLI),
	}

	if err := r.waitForRPC(ctx, 25*time.Second); err != nil {
		_ = r.Stop(context.Background())
		return nil, err
	}
	return r, nil
}

func connectExternalJunocashd(ctx context.Context, rpcURL string) (*RunningJunocashd, error) {
	parsed, err := url.Parse(rpcURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid JUNO_TEST_RPC_URL: %q", rpcURL)
	}
	hostPort := parsed.Host
	if parsed.Port() == "" {
		return nil, fmt.Errorf("invalid JUNO_TEST_RPC_URL (missing port): %q", rpcURL)
	}
	rpcPort, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return nil, fmt.Errorf("invalid JUNO_TEST_RPC_URL port: %q", parsed.Port())
	}

	rpcUser := strings.TrimSpace(os.Getenv("JUNO_TEST_RPC_USER"))
	if rpcUser == "" {
		rpcUser = "rpcuser"
	}
	rpcPass := strings.TrimSpace(os.Getenv("JUNO_TEST_RPC_PASS"))
	if rpcPass == "" {
		rpcPass = "rpcpass"
	}

	container := strings.TrimSpace(os.Getenv("JUNO_TEST_JUNOCASHD_CONTAINER"))
	if container == "" {
		return nil, errors.New("JUNO_TEST_JUNOCASHD_CONTAINER is required when JUNO_TEST_RPC_URL is set")
	}
	datadir := strings.TrimSpace(os.Getenv("JUNO_TEST_JUNOCASHD_DATADIR"))
	if datadir == "" {
		datadir = "/data"
	}
	internalRPCPort := 8232
	if v := strings.TrimSpace(os.Getenv("JUNO_TEST_JUNOCASHD_RPC_PORT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			internalRPCPort = n
		}
	}

	r := &RunningJunocashd{
		cmd:             nil,
		Datadir:         datadir,
		RPCURL:          (&url.URL{Scheme: parsed.Scheme, Host: hostPort}).String(),
		RPCPort:         rpcPort,
		RPCUser:         rpcUser,
		RPCPassword:     rpcPass,
		cliPath:         "junocash-cli",
		dockerContainer: container,
		dockerDatadir:   datadir,
		dockerRPCPort:   internalRPCPort,
	}
	if err := r.waitForRPC(ctx, 25*time.Second); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RunningJunocashd) Stop(ctx context.Context) error {
	var err error
	r.stopOnce.Do(func() {
		if r.dockerContainer != "" && r.cmd == nil {
			err = nil
			return
		}
		if r.cmd == nil || r.cmd.Process == nil {
			err = nil
			return
		}

		// Prefer a graceful stop.
		if runtime.GOOS == "windows" {
			_ = r.cmd.Process.Kill()
		} else {
			_ = r.cmd.Process.Signal(syscall.SIGTERM)
		}

		done := make(chan struct{})
		go func() {
			_ = r.cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			_ = r.cmd.Process.Kill()
			_ = r.cmd.Wait()
		}

		_ = os.RemoveAll(r.Datadir)
	})
	return err
}

func (r *RunningJunocashd) CLICommand(ctx context.Context, args ...string) *exec.Cmd {
	if r.dockerContainer != "" {
		base := []string{
			"exec",
			r.dockerContainer,
			r.cliPath,
			"-regtest",
			"-datadir=" + r.dockerDatadir,
			"-rpcuser=" + r.RPCUser,
			"-rpcpassword=" + r.RPCPassword,
			"-rpcport=" + fmt.Sprint(r.dockerRPCPort),
		}
		return exec.CommandContext(ctx, "docker", append(base, args...)...)
	}

	base := []string{
		"-regtest",
		"-datadir=" + r.Datadir,
		"-rpcuser=" + r.RPCUser,
		"-rpcpassword=" + r.RPCPassword,
		"-rpcport=" + fmt.Sprint(r.RPCPort),
	}
	return exec.CommandContext(ctx, r.cliPath, append(base, args...)...)
}

func (r *RunningJunocashd) waitForRPC(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cli := junocashd.New(r.RPCURL, r.RPCUser, r.RPCPassword)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		_, err := cli.GetBlockchainInfo(ctx)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("junocashd rpc not ready: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func defaultCLIPath(cli string) string {
	if strings.TrimSpace(cli) != "" {
		return cli
	}
	return "junocash-cli"
}
