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

	Example:
		aah run --envprofile qa
		aah run --envprofile qa --config /path/to/config/external.conf

	Note: For production use, it is recommended to follow build and deploy approach. DO NOT USE 'aah run'.`,
	Flags: []console.Flag{
		console.StringFlag{
			Name:  "envprofile, e",
			Usage: "Environment profile name to activate (e.g: dev, qa, prod)",
			Value: "dev",
		},
		console.StringFlag{
			Name:  "config, c",
			Usage: "External config `FILE` for adding or overriding 'config/**/*.conf' values",
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
	appStartArgs := []string{"run"}

	configPath := absPath(c.String("config"))
	if !ess.IsStrEmpty(configPath) {
		appStartArgs = append(appStartArgs, "--config", configPath)
	}
	envProfile := c.String("envprofile")
	appStartArgs = append(appStartArgs, "--envprofile", envProfile)

	app := aah.App()
	if err := app.InitForCLI(importPath); err != nil {
		logFatal(err)
	}
	projectCfg := aahProjectCfg(app.BaseDir())
	cliLog = initCLILogger(projectCfg)
	checkAndGenerateInitgoFile(importPath, app.BaseDir())
	cliLog.Infof("Loaded aah project file: %s", filepath.Join(app.BaseDir(), aahProjectIdentifier))

	// Hot-Reload is applicable only to `dev` environment profile.
	if projectCfg.BoolDefault("hot_reload.enable", true) && envProfile == "dev" {
		cliLog.Infof("Hot-Reload enabled for environment profile: %s", envProfile)

		address := app.HTTPAddress()
		proxyPort := findAvailablePort()
		scheme := "http"
		if app.IsSSLEnabled() {
			scheme = "https"
		}
		appStartArgs = append(appStartArgs, "--proxyport", proxyPort)

		appURL, _ := url.Parse(fmt.Sprintf("%s://%s:%s", scheme, address, proxyPort))
		appHotReload := &hotReload{
			ProxyURL:      appURL,
			ProxyPort:     proxyPort,
			BaseDir:       app.BaseDir(),
			Addr:          address,
			Port:          app.HTTPPort(),
			IsSSL:         app.IsSSLEnabled(),
			SSLCert:       app.Config().StringDefault("server.ssl.cert", ""),
			SSLKey:        app.Config().StringDefault("server.ssl.key", ""),
			Args:          appStartArgs,
			Proxy:         httputil.NewSingleHostReverseProxy(appURL),
			ProjectConfig: projectCfg,
		}
		appHotReload.Watcher = &fswatcher{
			hr:             appHotReload,
			IgnoreDirList:  make(map[string]bool),
			IgnoreFileList: make(map[string]bool),
		}
		appHotReload.Start()
		return nil
	}

	cliLog.Info("Hot-Reload is not enabled, possibly 'hot_reload.enable = false' or environment profile is not 'dev'")
	cleanupAutoGenFiles(app.BaseDir())

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
	Watcher        *fswatcher
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
	cleanupAutoGenFiles(hr.BaseDir)
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
	if !hr.Watcher.running {
		go hr.Watcher.Start()
	}
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

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// fswatcher for aah hot-reload
//___________________________________

type fswatcher struct {
	running        bool
	w              *watcher.Watcher
	hr             *hotReload
	IgnoreFileList map[string]bool
	IgnoreDirList  map[string]bool
}

func (fs *fswatcher) Start() {
	if fs.w != nil {
		return
	}
	fs.w = watcher.New()
	watch := make(chan bool)
	go func() {
		for {
			fs.hr.ChangedOrError = <-watch
		}
	}()
	fs.w.IgnoreHiddenFiles(true)
	// w.w.SetMaxEvents(1)
	fs.w.FilterOps(watcher.Create, watcher.Write, watcher.Remove, watcher.Rename, watcher.Move)
	fs.AddAppFiles()

	go func() {
		for {
			select {
			case e := <-fs.w.Event:
				if !fs.IsInIgnoreList(e) {
					if e.Op == watcher.Create || e.Op == watcher.Rename || e.Op == watcher.Move {
						_ = fs.w.Add(e.Path)
					}
					watch <- true
				}
			case err := <-fs.w.Error:
				if err == watcher.ErrWatchedFileDeleted {
					watch <- true
				}
			case <-fs.w.Closed:
				return
			}
		}
	}()

	if cliLog.IsLevelTrace() {
		var fileList []string
		for path := range fs.w.WatchedFiles() {
			fileList = append(fileList, stripGoSrcPath(path))
		}
		cliLog.Trace("Watched files:\n\t", strings.Join(fileList, "\n\t"))
	}

	go func() { fs.w.Wait() }()
	fs.running = true
	if err := fs.w.Start(time.Millisecond * 100); err != nil {
		fs.running = false
		logError(err)
	}
}

// AddWatch method adds files into watcher and create app watch ignore list.
func (fs *fswatcher) AddAppFiles() {
	// Build ignore list using User provided list via config plus defaults
	dirExcludes, _ := fs.hr.ProjectConfig.StringList("hot_reload.watch.dir_excludes")
	dirExcludes = append(dirExcludes, "build", "static", "vendor", "views", "tests", "logs") // put defaults
	for _, d := range dirExcludes {
		fs.IgnoreDirList[filepath.Join(fs.hr.BaseDir, filepath.FromSlash(d))] = true
	}
	fs.IgnoreDirList[filepath.Join(fs.hr.BaseDir, filepath.FromSlash("app/generated"))] = true

	fileExcludes, _ := fs.hr.ProjectConfig.StringList("hot_reload.watch.file_excludes")
	fileExcludes = append(fileExcludes, ".*", "*.pid", "*_test.go", "LICENSE", "README.md") // put defaults
	for _, f := range fileExcludes {
		fs.IgnoreFileList[filepath.Join(fs.hr.BaseDir, filepath.FromSlash(f))] = true
	}
	fs.IgnoreFileList[filepath.Join(fs.hr.BaseDir, "app", "aah.go")] = true
	fs.IgnoreFileList[filepath.Join(fs.hr.BaseDir, "app", "aah*_vfs.go")] = true

	dirs, _ := ess.DirsPathExcludes(fs.hr.BaseDir, true, append(dirExcludes, "generated"))
	for _, d := range dirs {
		if err := fs.w.Add(d); err != nil {
			logErrorf("Unable add watch for '%v'", d)
		}
		files, _ := ess.FilesPathExcludes(d, false, append(fileExcludes, "aah.go", "aah*_vfs.go"))
		for _, f := range files {
			if err := fs.w.Add(f); err != nil {
				logErrorf("Unable add watch for '%v'", f)
			}
		}
	}
	var err error
	for _, f := range fileExcludes {
		if err = fs.w.Ignore(f); err != nil {
			logError(err)
		}
	}
}

func (fs *fswatcher) IsInIgnoreList(e watcher.Event) bool {
	appDir := filepath.Join(fs.hr.BaseDir, "app")
	if fs.hr.BaseDir == e.Path || appDir == e.Path {
		return true
	}
	if e.IsDir() {
		for k := range fs.IgnoreDirList {
			if strings.HasPrefix(e.Path, k) {
				return true
			}
		}
	} else {
		for k := range fs.IgnoreFileList {
			if matched, _ := filepath.Match(k, e.Path); matched {
				return true
			}
		}
	}
	return false
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// process and its methods
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
			return
		}
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
	if proc, err := os.FindProcess(p.cmd.Process.Pid); err == nil {
		_ = proc.Kill()
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
