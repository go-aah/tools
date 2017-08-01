// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/tools/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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

	"gopkg.in/radovskyb/watcher.v1"
	"gopkg.in/urfave/cli.v1"

	"aahframework.org/aah.v0-unstable"
	"aahframework.org/config.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

var runCmd = cli.Command{
	Name:    "run",
	Aliases: []string{"r"},
	Usage:   "Run aah framework application (supports hot-reload)",
	Description: `Run the aah framework web/api application. It supports hot-reload, just code and refresh the browser
	to see your updates.

	Examples of short and long flags:
    aah run
		aah run -e qa

		aah run -i github.com/user/appname
		aah run -i github.com/user/appname -e qa
		aah run -i github.com/user/appname -e qa -c /path/to/config/external.conf

    aah run --importpath github.com/username/name
		aah run --importpath github.com/username/name --envprofile qa
		aah run --importpath github.com/username/name --envprofile qa --config /path/to/config/external.conf

	Note: It is recommended to use build and deploy approach instead of
	using 'aah run' for production use.`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "i, importpath",
			Usage: "Import path of aah application",
		},
		cli.StringFlag{
			Name:  "e, envprofile",
			Usage: "Environment profile name to activate. e.g: dev, qa, prod"},
		cli.StringFlag{
			Name:  "c, config",
			Usage: "External config file for overriding aah.conf values",
		},
	},
	Action: runAction,
}

type (
	proxy struct {
		ProxyURL       *url.URL
		ProxyPort      string
		BaseDir        string
		Addr           string
		Port           string
		IsSSL          bool
		SSLCert        string
		SSLKey         string
		Args           []string
		Server         *httputil.ReverseProxy
		Process        *process
		ProjectConfig  *config.Config
		ChangedOrError bool
		Watcher        *watcher.Watcher
	}

	process struct {
		cmd *exec.Cmd
		nw  *notifyWriter
	}

	notifyWriter struct {
		w          io.Writer
		checkBytes []byte
		notify     chan bool
	}
)

func runAction(c *cli.Context) error {
	importPath := firstNonEmpty(c.String("i"), c.String("importpath"))
	if ess.IsStrEmpty(importPath) {
		importPath = importPathRelwd()
	}

	if !ess.IsImportPathExists(importPath) {
		fatalf("Given import path '%s' does not exists", importPath)
	}

	appStartArgs := []string{}
	configPath := getNonEmptyAbsPath(c.String("c"), c.String("config"))
	if !ess.IsStrEmpty(configPath) {
		appStartArgs = append(appStartArgs, "-config", configPath)
	}

	envProfile := firstNonEmpty(c.String("e"), c.String("envprofile"), "dev")
	if !ess.IsStrEmpty(envProfile) {
		appStartArgs = append(appStartArgs, "-profile", envProfile)
	}

	aah.Init(importPath)
	projectCfg, err := loadAahProjectFile(aah.AppBaseDir())
	if err != nil {
		fatalf("aah project file error: %s", err)
	}

	// Hot-Reload is applicable only to `dev` environment profile.
	if projectCfg.BoolDefault("hot_reload.enable", true) && envProfile == "dev" {
		log.Infof("Hot-Reload enabled for environment profile: %s", aah.AppProfile())

		address := firstNonEmpty(aah.AppHTTPAddress(), "localhost")
		proxyPort := findAvailablePort()
		scheme := "http"
		if aah.AppIsSSLEnabled() {
			scheme = "https"
		}

		appURL, _ := url.Parse(fmt.Sprintf("%s://%s:%s", scheme, address, proxyPort))
		appProxy := &proxy{
			ProxyURL:      appURL,
			ProxyPort:     proxyPort,
			BaseDir:       aah.AppBaseDir(),
			Addr:          address,
			Port:          aah.AppHTTPPort(),
			IsSSL:         aah.AppIsSSLEnabled(),
			SSLCert:       aah.AppConfig().StringDefault("server.ssl.cert", ""),
			SSLKey:        aah.AppConfig().StringDefault("server.ssl.key", ""),
			Args:          appStartArgs,
			Server:        httputil.NewSingleHostReverseProxy(appURL),
			ProjectConfig: projectCfg,
		}

		appProxy.Start()
		return nil
	}

	log.Info("Hot-Reload is not enabled, possibly 'hot_reload.enable = false' or environment profile is not 'dev'")

	appBinary, err := compileApp(&compileArgs{
		Cmd:        "RunCmd",
		ProjectCfg: projectCfg,
		AppPack:    false,
	})
	if err != nil {
		fatal(err)
	}

	if _, err := execCmd(appBinary, appStartArgs, true); err != nil {
		fatal(err)
	}

	return nil
}

func (p *proxy) Start() {
	// starting proxy server
	go func() {
		p.Server.ErrorLog = log.ToGoLogger()
		p.Server.ErrorLog.SetOutput(ioutil.Discard)

		var err error
		address := fmt.Sprintf("%s:%s", p.Addr, p.Port)
		server := &http.Server{Addr: address, Handler: p}
		server.ErrorLog = p.Server.ErrorLog

		if p.IsSSL {
			p.Server.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			err = server.ListenAndServeTLS(p.SSLCert, p.SSLKey)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil {
			fatalf("Unable to start proxy server, %s", err.Error())
		}
	}()

	if err := p.CompileAndStart(); err != nil {
		fatal(err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc
	p.Stop()
}

func (p *proxy) CompileAndStart() error {
	appBinary, err := compileApp(&compileArgs{
		Cmd:        "RunCmd",
		ProxyPort:  p.ProxyPort,
		ProjectCfg: p.ProjectConfig,
		AppPack:    false,
	})
	if err != nil {
		return err
	}

	p.Process = &process{
		cmd: exec.Command(appBinary, p.Args...),
		nw: &notifyWriter{
			w:          os.Stdout,
			notify:     make(chan bool),
			checkBytes: []byte("aah go server running on"),
		},
	}

	if err = p.Process.Start(); err != nil {
		return err
	}

	p.RefreshWatcher()

	return nil
}

func (p *proxy) Stop() {
	p.Process.Stop()
}

func (p *proxy) RefreshWatcher() {
	p.Watcher = watcher.New()
	watch := make(chan bool)
	go startWatcher(p.ProjectConfig, p.BaseDir, p.Watcher, watch)
	go func() {
		for {
			p.ChangedOrError = <-watch
		}
	}()
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.ChangedOrError {
		log.Info("Application file change(s) detected")
		p.Watcher.Close()
		p.Stop()
		if err := p.CompileAndStart(); err != nil {
			log.Error(err)
			fmt.Fprintln(w, err.Error())
			return
		}
		p.ChangedOrError = false
	}
	p.Server.ServeHTTP(w, r)
}

func startWatcher(projectCfg *config.Config, baseDir string, w *watcher.Watcher, watch chan<- bool) {
	w.IgnoreHiddenFiles(true)
	w.SetMaxEvents(1)

	loadWatchFiles(projectCfg, baseDir, w)

	go func() { w.Wait() }()

	go func() {
		for {
			select {
			case e := <-w.Event:
				if e.Op == watcher.Create {
					_ = w.Add(e.Path)
				}
				watch <- true
			case err := <-w.Error:
				if err == watcher.ErrWatchedFileDeleted {
					// treat as trace information, not an error
					log.Trace("Watched file/directory is deleted, just move on")
				}
			case <-w.Closed:
				return
			}
		}
	}()

	if log.IsLevelTrace() {
		var fileList []string
		for path := range w.WatchedFiles() {
			fileList = append(fileList, stripGoPath(path))
		}
		log.Trace("Watched files:\n\t", strings.Join(fileList, "\n\t"))
	}

	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Error(err)
	}
}

func loadWatchFiles(projectCfg *config.Config, baseDir string, w *watcher.Watcher) {
	// standard file ignore list for aah project
	stdIgnoreList := []string{
		filepath.Join(baseDir, aah.AppName()+".pid"),
		filepath.Join(baseDir, "app", "aah.go"),
	}

	// user can provide their list via config
	dirExcludes, _ := projectCfg.StringList("hot_reload.watch.dir_excludes")
	if len(dirExcludes) == 0 { // put defaults
		dirExcludes = append(dirExcludes, ".*")
	}

	fileExcludes, _ := projectCfg.StringList("hot_reload.watch.file_excludes")
	if len(fileExcludes) == 0 { // put defaults
		fileExcludes = append(fileExcludes, ".*", "_test.go", "LICENSE", "README.md")
	}

	// standard dir ignore list for aah project
	dirExcludes = append(dirExcludes, "build", "static", "vendor", "tests", "logs")

	dirs, _ := ess.DirsPathExcludes(baseDir, true, dirExcludes)
	// dirs = excludeAndCreateSlice(dirs, baseDir)
	// dirs = excludeAndCreateSlice(dirs, filepath.Join(baseDir, "app"))
	for _, d := range dirs {
		if err := w.Add(d); err != nil {
			log.Errorf("Unable add watch for '%v'", d)
		}

		files, _ := ess.FilesPathExcludes(d, false, fileExcludes)
		for _, f := range files {
			if err := w.Add(f); err != nil {
				log.Errorf("Unable add watch for '%v'", f)
			}
		}
	}

	// Add ignore list
	if err := w.Ignore(stdIgnoreList...); err != nil {
		log.Error(err)
	}
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// process methods
//___________________________________

func (p *process) Start() error {
	log.Debug("Executing ", strings.Join(p.cmd.Args, " "))
	p.cmd.Stdout = p.nw
	p.cmd.Stderr = p.nw
	if err := p.cmd.Start(); err != nil {
		return err
	}

	for {
		select {
		case <-p.nw.notify:
			return nil
		case <-p.processWait():
			return errors.New("aah application did not start")
		}
	}
}

func (p *process) Stop() {
	if p.cmd != nil && (p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited()) {
		if isWindowsOS() {
			// For windows console app, no graceful close is available;
			// so we have only option is to kill.
			_ = p.cmd.Process.Kill()
		} else {
			p.nw.checkBytes = []byte("application stopped")
			p.nw.notify = make(chan bool)
			_ = p.cmd.Process.Signal(os.Interrupt)
			// wait for process to finish or return after grace time
			for {
				select {
				case <-p.nw.notify:
					return
				case <-time.After(time.Millisecond * 300):
					return
				}
			}
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

func (nw notifyWriter) Write(b []byte) (n int, err error) {
	if nw.notify != nil && bytes.Contains(b, nw.checkBytes) {
		nw.notify <- true
		nw.notify = nil
	}
	return nw.w.Write(b)
}
