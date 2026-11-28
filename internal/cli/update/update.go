package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/paths"
)

const (
	usage         = "gote update [--check] [--force] [--json]"
	repo          = "banorton/gote"
	maxDownload   = 200 << 20
	sumsAssetName = "SHA256SUMS"
)

// Package-level so tests can point at a fake GitHub; deliberately not
// configurable from env or user config.
var (
	apiBase        = "https://api.github.com"
	apiClient      = &http.Client{Timeout: 15 * time.Second}
	downloadClient = &http.Client{Timeout: 10 * time.Minute}
	executable     = os.Executable
)

// serverHooks are the seams for talking to the background server; tests replace them.
type serverHooks struct {
	Socket   func() (string, error)
	Probe    func(socket string) (client.ServerInfo, error)
	Shutdown func(socket string) error
	WaitGone func(socket string, d time.Duration) error
	// Start spawns exe as the server and waits until it reports version.
	Start func(exe, socket, version string) error
	// Enabled reports whether this user's config and flags allow a server.
	Enabled func(c *cli.Ctx) (bool, error)
}

var hooks = serverHooks{
	Socket:   paths.SocketPath,
	Probe:    client.Probe,
	Shutdown: client.Shutdown,
	WaitGone: client.WaitGone,
	Start: func(exe, socket, version string) error {
		state, err := paths.StateDir()
		if err != nil {
			return err
		}
		_, err = client.Connect(client.ConnectOpts{
			Socket: socket, Version: version,
			Spawn: func() error { return client.SpawnBinary(exe, filepath.Join(state, "server.log")) },
		})
		return err
	},
	Enabled: func(c *cli.Ctx) (bool, error) {
		if c.NoServer {
			return false, nil
		}
		cfg, err := config.Load()
		if err != nil {
			return false, err
		}
		return cfg.Server, nil
	},
}

var cleanVersion = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

func init() {
	cli.Register(cli.Command{
		Name:          "update",
		Summary:       "update gote to the latest GitHub release",
		Usage:         usage,
		NoConfig:      true,
		SkipReconcile: true,
		Run:           run,
	})
}

type release struct {
	Tag    string `json:"tag_name"`
	URL    string `json:"html_url"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (r *release) asset(name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

func parseSemver(v string) ([3]int, error) {
	m := cleanVersion.FindStringSubmatch(v)
	if m == nil {
		return [3]int{}, fmt.Errorf("%q is not a vX.Y.Z version", v)
	}
	var out [3]int
	for i := range out {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return out, fmt.Errorf("%q: %w", v, err)
		}
		out[i] = n
	}
	return out, nil
}

// compareSemver returns -1, 0 or 1 for a<b, a==b, a>b.
func compareSemver(a, b string) (int, error) {
	pa, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := range pa {
		switch {
		case pa[i] < pb[i]:
			return -1, nil
		case pa[i] > pb[i]:
			return 1, nil
		}
	}
	return 0, nil
}

func run(c *cli.Ctx, args []string) error {
	fs := c.FlagSet("update")
	check := fs.Bool("check", false, "only report whether an update is available")
	force := fs.Bool("force", false, "update a development build, or reinstall the latest release")
	if err := cli.ParseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return cli.Usagef("update takes no arguments; usage: %s", usage)
	}
	if *check {
		return runCheck(c)
	}
	return runUpdate(c, *force)
}

func fetchLatest() (*release, error) {
	req, err := http.NewRequest(http.MethodGet, apiBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gote/"+cli.Version)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch latest release: GitHub returned %s", resp.Status)
	}
	var rel release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode latest release: %w", err)
	}
	if _, err := parseSemver(rel.Tag); err != nil {
		return nil, fmt.Errorf("latest release tag: %w", err)
	}
	return &rel, nil
}

// newer reports whether latest is strictly newer than current. A current
// version that is not a clean release tag cannot be compared and counts as outdated.
func newer(latest, current string) bool {
	cmp, err := compareSemver(latest, current)
	return err != nil || cmp > 0
}

func runCheck(c *cli.Ctx) error {
	rel, err := fetchLatest()
	if err != nil {
		return err
	}
	avail := newer(rel.Tag, cli.Version)
	if c.JSON {
		return c.PrintJSON(map[string]any{
			"current": cli.Version, "latest": rel.Tag, "update_available": avail, "url": rel.URL,
		})
	}
	if avail {
		fmt.Fprintf(c.Out, "gote %s → %s available\n", cli.Version, rel.Tag)
	} else {
		fmt.Fprintf(c.Out, "gote %s is up to date\n", cli.Version)
	}
	return nil
}

func runUpdate(c *cli.Ctx, force bool) error {
	if !cleanVersion.MatchString(cli.Version) && !force {
		return fmt.Errorf("gote %s is a development build; update only replaces release builds. Use `git pull && make build`, or pass --force to install the latest release anyway", cli.Version)
	}
	exe, err := executable()
	if err != nil {
		return fmt.Errorf("locate running binary: %w", err)
	}
	target, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", exe, err)
	}
	rel, err := fetchLatest()
	if err != nil {
		return err
	}
	if !force && !newer(rel.Tag, cli.Version) {
		if c.JSON {
			return c.PrintJSON(map[string]string{"previous": cli.Version, "current": cli.Version, "path": target})
		}
		fmt.Fprintf(c.Out, "gote %s is already up to date\n", cli.Version)
		return nil
	}

	name := fmt.Sprintf("gote-%s-%s", runtime.GOOS, runtime.GOARCH)
	assetURL := rel.asset(name)
	if assetURL == "" {
		var names []string
		for _, a := range rel.Assets {
			names = append(names, a.Name)
		}
		return fmt.Errorf("release %s has no asset %s; available: %s", rel.Tag, name, strings.Join(names, ", "))
	}
	sumsURL := rel.asset(sumsAssetName)
	if sumsURL == "" {
		return fmt.Errorf("release %s has no %s asset; refusing to install unverified binary", rel.Tag, sumsAssetName)
	}
	sums, err := downloadBytes(sumsURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", sumsAssetName, err)
	}
	want, err := checksumFor(string(sums), name)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".gote-update-*")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("cannot write next to %s (%w); re-run with sufficient permissions", target, err)
		}
		return err
	}
	tmpPath := tmp.Name()
	installed := false
	defer func() {
		if !installed {
			os.Remove(tmpPath)
		}
	}()

	got, err := download(assetURL, tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: %s says %s, downloaded file is %s", name, sumsAssetName, want, got)
	}

	mode := os.FileMode(0o755)
	if fi, err := os.Stat(target); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	if err := sanityCheck(tmpPath, rel.Tag); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("cannot replace %s (%w); re-run with sufficient permissions", target, err)
		}
		return err
	}
	installed = true

	state, msg := refreshServer(c, target, rel.Tag)
	if c.JSON {
		return c.PrintJSON(map[string]string{"previous": cli.Version, "current": rel.Tag, "path": target, "server": state})
	}
	fmt.Fprintf(c.Out, "updated gote %s → %s (%s)\n", cli.Version, rel.Tag, target)
	if msg != "" {
		fmt.Fprintln(c.Out, msg)
	}
	return nil
}

// refreshServer replaces a running old-version server with the new binary,
// unless a web tab is open (a restart gives the web view a new port and token).
// It returns the JSON state and a human message. Failures are warnings on
// stderr: the update itself already succeeded.
func refreshServer(c *cli.Ctx, exe, newVersion string) (state, msg string) {
	warn := func(err error) (string, string) {
		fmt.Fprintf(c.Err, "gote: warning: server not restarted: %v; it restarts on your next command\n", err)
		return "kept", ""
	}
	sock, err := hooks.Socket()
	if err != nil {
		return warn(err)
	}
	info, err := hooks.Probe(sock)
	if err != nil {
		return "not_running", ""
	}
	if info.WebClients > 0 {
		return "kept", fmt.Sprintf("server still running %s: an editor or note tab is open; it restarts on your next command, or run gote server stop", info.Version)
	}
	enabled, err := hooks.Enabled(c)
	if err != nil {
		return warn(err)
	}
	if err := hooks.Shutdown(sock); err != nil {
		return warn(fmt.Errorf("stop server %s: %w", info.Version, err))
	}
	if err := hooks.WaitGone(sock, 3*time.Second); err != nil {
		return warn(err)
	}
	if !enabled {
		return "stopped", fmt.Sprintf("stopped gote server %s (server is disabled in config or by --no-server)", info.Version)
	}
	if err := hooks.Start(exe, sock, newVersion); err != nil {
		warn(fmt.Errorf("start server: %w", err))
		return "not_running", ""
	}
	return "restarted", "restarted gote server on " + newVersion
}

func checksumFor(sums, name string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s has no line for %s; refusing to install unverified binary", sumsAssetName, name)
}

func get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gote/"+cli.Version)
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GitHub returned %s", resp.Status)
	}
	return resp, nil
}

// download streams url into w and returns the hex sha256 of what was written.
func download(url string, w io.Writer) (string, error) {
	resp, err := get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return "", err
	}
	if n > maxDownload {
		return "", fmt.Errorf("exceeds %d MB size cap", maxDownload>>20)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func downloadBytes(url string) ([]byte, error) {
	resp, err := get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func sanityCheck(path, tag string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return fmt.Errorf("downloaded binary failed `--version`: %w", err)
	}
	if got, want := strings.TrimSpace(string(out)), "gote "+tag; got != want {
		return fmt.Errorf("downloaded binary reports %q, expected %q; not installing", got, want)
	}
	return nil
}
