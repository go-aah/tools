// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"aahframe.work"
	"aahframe.work/config"
	"aahframe.work/console"
	"aahframe.work/essentials"

	"github.com/radovskyb/watcher"
)

var runCmd = console.Command{
	Name:    "run",
	Aliases: []string{"r"},
	Usage:   "Runs aah application (supports hot-reload)",
	Description: `Runs aah application. It supports hot-reload (just code and refresh the browser
	to see your updates).

	Examples of short and long flags:
    aah run
		aah run -e qa

		aah run
		aah run -e qa
		aah run -e qa -c /path/to/config/external.conf

    aah run
		aah run --envprofile qa
		aah run --envprofile qa --config /path/to/config/external.conf

	Note: For production use, it is recommended to follow build and deploy approach. Do not use 'aah run'.`,
	Flags: []console.Flag{
		console.StringFlag{
			Name:  "e, envprofile",
			Usage: "Environment profile name to activate (e.g: dev, qa, prod)"},
		console.StringFlag{
			Name:  "c, config",
			Usage: "External config file for overriding aah.conf values",
		},
	},
	Action: runAction,
}

func runAction(c *console.Context) error {
	if !isAahProject() {
		logFatalf("Please go to aah application base directory and run '%s'.", strings.Join(os.Args, " "))
	}

	importPath := appImportPath(c)
	if ess.IsStrEmpty(importPath) {
		logFatalf("Unable to infer import path, ensure you're in the application base directory")
	}
	chdirIfRequired(importPath)
	appStartArgs := []string{}

	configPath := getNonEmptyAbsPath(c.String("c"), c.String("config"))
	if !ess.IsStrEmpty(configPath) {
		appStartArgs = append(appStartArgs, "-config", configPath)
	}

	envProfile := firstNonEmpty(c.String("e"), c.String("envprofile"))
	if !ess.IsStrEmpty(envProfile) {
		appStartArgs = append(appStartArgs, "-profile", envProfile)
	}

	if err := aah.Init(importPath); err != nil {
		logFatal(err)
	}
	projectCfg := aahProjectCfg(aah.AppBaseDir())
	cliLog = initCLILogger(projectCfg)

	checkAndGenerateInitgoFile(importPath, aah.AppBaseDir())

	cliLog.Infof("Loaded aah project file: %s", filepath.Join(aah.AppBaseDir(), aahProjectIdentifier))

	if ess.IsStrEmpty(envProfile) {
		envProfile = aah.AppProfile()
	}

	// Hot-Reload is applicable only to `dev` environment profile.
	if projectCfg.BoolDefault("hot_reload.enable", true) && envProfile == "dev" {
		cliLog.Infof("Hot-Reload enabled for environment profile: %s", aah.AppProfile())

		address := firstNonEmpty(aah.AppHTTPAddress(), "")
		proxyPort := findAvailablePort()
		scheme := "http"
		if aah.AppIsSSLEnabled() {
			scheme = "https"
		}

		appURL, _ := url.Parse(fmt.Sprintf("%s://%s:%s", scheme, address, proxyPort))
		appHotReload := &hotReload{
			ProxyURL:      appURL,
			ProxyPort:     proxyPort,
			BaseDir:       aah.AppBaseDir(),
			Addr:          address,
			Port:          aah.AppHTTPPort(),
			IsSSL:         aah.AppIsSSLEnabled(),
			SSLCert:       aah.AppConfig().StringDefault("server.ssl.cert", ""),
			SSLKey:        aah.AppConfig().StringDefault("server.ssl.key", ""),
			Args:          appStartArgs,
			Proxy:         httputil.NewSingleHostReverseProxy(appURL),
			ProjectConfig: projectCfg,
		}

		appHotReload.Start()
		return nil
	}

	cliLog.Info("Hot-Reload is not enabled, possibly 'hot_reload.enable = false' or environment profile is not 'dev'")

	appBinary, err := compileApp(&compileArgs{
		Cmd:        "RunCmd",
		ProjectCfg: projectCfg,
		AppPack:    false,
		AppEmbed:   false,
	})
	if err != nil {
		logFatal(err)
	}

	if _, err := execCmd(appBinary, appStartArgs, true); err != nil {
		logFatal(err)
	}

	return nil
}

type hotReload struct {
	ChangedOrError bool
	IsSSL          bool
	ProxyPort      string
	BaseDir        string
	Addr           string
	Port           string
	SSLCert        string
	SSLKey         string
	Args           []string
	ProxyURL       *url.URL
	Proxy          *httputil.ReverseProxy
	Process        *process
	ProjectConfig  *config.Config
	Watcher        *watcher.Watcher
}

func (hr *hotReload) Start() {
	// Starting Hot-Reload server
	go func() {
		hr.Proxy.ErrorLog = cliLog.ToGoLogger()
		hr.Proxy.ErrorLog.SetOutput(ioutil.Discard)
		hr.Proxy.Transport = http.DefaultTransport

		var err error
		address := fmt.Sprintf("%s:%s", hr.Addr, hr.Port)
		server := &http.Server{
			Addr:         address,
			Handler:      hr,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}
		server.ErrorLog = hr.Proxy.ErrorLog

		if hr.IsSSL {
			/* #nosec Its required for development activity */
			hr.Proxy.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			err = server.ListenAndServeTLS(hr.SSLCert, hr.SSLKey)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil {
			logFatalf("Unable to start aah dev hot-reload server, %s", err.Error())
		}
	}()

	if err := hr.CompileAndStart(); err != nil {
		logFatal(err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc
	hr.Stop()
}

func (hr *hotReload) CompileAndStart() error {
	appBinary, err := compileApp(&compileArgs{
		Cmd:        "RunCmd",
		ProxyPort:  hr.ProxyPort,
		ProjectCfg: hr.ProjectConfig,
		AppPack:    false,
		AppEmbed:   false,
	})
	if err != nil {
		return err
	}

	hr.Process = &process{
		// #nosec
		cmd: exec.Command(appBinary, hr.Args...),
		nw: &notifyWriter{
			w:          os.Stdout,
			notify:     make(chan bool),
			checkBytes: []byte("aah go server running"),
		},
	}

	go hr.StartWatcher()
	return hr.Process.Start()
}

func (hr *hotReload) Stop() {
	hr.Process.Stop()
}

func (hr *hotReload) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if hr.ChangedOrError {
		cliLog.Info("Application file change(s) detected")
		hr.ChangedOrError = false
		hr.Stop()
		if err := hr.CompileAndStart(); err != nil {
			logError(err)
			fmt.Fprintln(w, err.Error())
			hr.ChangedOrError = true
			return
		}
		waitForConnReady(hr.ProxyPort)
	}
	hr.ProxyServe(w, r)
}

// Typically for HTTP method: CONNECT and WebSocket needs tunneling, we cannot
// use `httputil.ReverseProxy` since it handles Hop-By-Hop headers on proxy
// connection - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers#hbh
func (hr *hotReload) needTunneling(r *http.Request) bool {
	return r.Method == http.MethodConnect ||
		strings.EqualFold(strings.ToLower(r.Header.Get("Upgrade")), "websocket")
}

func (hr *hotReload) ProxyServe(w http.ResponseWriter, r *http.Request) {
	if hr.needTunneling(r) {
		hr.tunnel(w, r)
	} else {
		hr.Proxy.ServeHTTP(w, r)
	}
}

func (hr *hotReload) tunnel(w http.ResponseWriter, r *http.Request) {
	var peer net.Conn
	var err error
	address := fmt.Sprintf("%s:%s", hr.Addr, hr.ProxyPort)
	if hr.IsSSL {
		/* #nosec Its required for development activity */
		peer, err = tls.Dial("tcp", address, &tls.Config{InsecureSkipVerify: true})
	} else {
		peer, err = net.DialTimeout("tcp", address, 10*time.Second)
	}

	if err != nil {
		http.Error(w, "Error tunneling with peer", http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Error hijacking is not supported", http.StatusInternalServerError)
		return
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err = r.Write(peer); err != nil {
		logErrorf("Error tunneling data to peer: %s", err)
		return
	}

	go func() {
		defer ess.CloseQuietly(peer)
		defer ess.CloseQuietly(conn)
		_, _ = io.Copy(peer, conn)
	}()
	go func() {
		defer ess.CloseQuietly(conn)
		defer ess.CloseQuietly(peer)
		_, _ = io.Copy(conn, peer)
	}()
}

func (hr *hotReload) StartWatcher() {
	if hr.Watcher != nil {
		return
	}
	hr.Watcher = watcher.New()
	watch := make(chan bool)
	go func() {
		for {
			hr.ChangedOrError = <-watch
		}
	}()
	hr.Watcher.IgnoreHiddenFiles(true)
	hr.Watcher.SetMaxEvents(1)
	hr.Watcher.FilterOps(watcher.Create, watcher.Write, watcher.Remove, watcher.Rename, watcher.Move)
	hr.AddWatchFiles()

	appDir := filepath.Join(hr.BaseDir, "app")
	go func() {
		for {
			select {
			case e := <-hr.Watcher.Event:
				if !strings.EqualFold(filepath.Ext(e.Path), ".pid") && hr.BaseDir != e.Path && appDir != e.Path {
					if e.Op == watcher.Create || e.Op == watcher.Rename || e.Op == watcher.Move {
						_ = hr.Watcher.Add(e.Path)
					} else if e.Op == watcher.Remove {
						_ = hr.Watcher.Remove(e.Path)
					}
					watch <- true
				}
			case err := <-hr.Watcher.Error:
				if err == watcher.ErrWatchedFileDeleted {
					watch <- true
				}
			case <-hr.Watcher.Closed:
				return
			}
		}
	}()

	if cliLog.IsLevelTrace() {
		var fileList []string
		for path := range hr.Watcher.WatchedFiles() {
			fileList = append(fileList, stripGoSrcPath(path))
		}
		cliLog.Trace("Watched files:\n\t", strings.Join(fileList, "\n\t"))
	}

	go func() { hr.Watcher.Wait() }()
	if err := hr.Watcher.Start(time.Millisecond * 100); err != nil {
		logError(err)
	}
}

func (hr *hotReload) AddWatchFiles() {
	pidName := hr.ProjectConfig.StringDefault("build.binary_name", aah.AppName())
	// standard file ignore list for aah project
	stdIgnoreList := []string{
		filepath.Join(hr.BaseDir, pidName+".pid"),
		filepath.Join(hr.BaseDir, "app", "aah.go"),
		filepath.Join(hr.BaseDir, "app", "aah*_vfs.go"),
		filepath.Join(hr.BaseDir, "build"),
	}

	// user can provide their list via config
	dirExcludes, _ := hr.ProjectConfig.StringList("hot_reload.watch.dir_excludes")
	if len(dirExcludes) == 0 { // put defaults
		dirExcludes = append(dirExcludes, ".*")
	}

	fileExcludes, _ := hr.ProjectConfig.StringList("hot_reload.watch.file_excludes")
	if len(fileExcludes) == 0 { // put defaults
		fileExcludes = append(fileExcludes, ".*", "_test.go", "LICENSE", "README.md")
	}

	// standard dir ignore list for aah project
	dirExcludes = append(dirExcludes, "build", "static", "vendor", "views", "tests", "logs")

	dirs, _ := ess.DirsPathExcludes(hr.BaseDir, true, dirExcludes)
	for _, d := range dirs {
		if err := hr.Watcher.Add(d); err != nil {
			logErrorf("Unable add watch for '%v'", d)
		}

		files, _ := ess.FilesPathExcludes(d, false, fileExcludes)
		for _, f := range files {
			if err := hr.Watcher.Add(f); err != nil {
				logErrorf("Unable add watch for '%v'", f)
			}
		}
	}

	// Add ignore list
	if err := hr.Watcher.Ignore(stdIgnoreList...); err != nil {
		logError(err)
	}
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// process methods
//___________________________________

type process struct {
	cmd *exec.Cmd
	nw  *notifyWriter
}

func (p *process) Start() error {
	cliLog.Debug("Executing ", strings.Join(p.cmd.Args, " "))
	p.cmd.Stdout = p.nw
	p.cmd.Stderr = p.nw
	if err := p.cmd.Start(); err != nil {
		return err
	}

	select {
	case <-p.nw.notify:
		return nil
	case <-p.processWait():
		return errors.New("aah application did not start")
	}
}

func (p *process) Stop() {
	if p.cmd != nil && (p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited()) {
		if isWindowsOS() {
			// For windows console app, no graceful close is available;
			// so we have only option is to kill.
			_ = p.cmd.Process.Kill()
		} else {
			p.nw.checkBytes = []byte("shutdown successful")
			p.nw.notify = make(chan bool)
			_ = p.cmd.Process.Signal(os.Interrupt)
			// wait for process to finish or return after grace time
			select {
			case <-p.nw.notify:
				return
			case <-time.After(time.Millisecond * 300):
				return
			}
		}
	} else {
		proc, err := os.FindProcess(p.cmd.Process.Pid)
		if err == nil {
			_ = proc.Kill()
		}
	}
}

func (p *process) processWait() <-chan bool {
	wait := make(chan bool)
	go func() {
		_ = p.cmd.Wait()
		wait <- true
	}()
	return wait
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// notifyWriter methods
//___________________________________

type notifyWriter struct {
	w          io.Writer
	checkBytes []byte
	notify     chan bool
}

func (nw *notifyWriter) Write(b []byte) (n int, err error) {
	if nw.notify != nil && bytes.Contains(b, nw.checkBytes) {
		nw.notify <- true
		nw.notify = nil
	}
	return nw.w.Write(b)
}
