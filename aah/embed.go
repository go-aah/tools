// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"aahframe.work"
	"aahframe.work/essentials"
	"aahframe.work/vfs"
)

// Standard frame type MTU size is 1500 bytes so 1400 bytes would make sense
// to Gzip by default. Read: https://en.wikipedia.org/wiki/Maximum_transmission_unit
var defaultGzipMinSize int64 = 1400

var vfsTmpl = template.Must(template.New("vfs").Funcs(vfsTmplFuncMap).Parse(vfsTmplStr))

func processMount(mode bool, appBaseDir, vroot, proot string, skipList ess.Excludes, noGzipList []string) error {
	proot = filepath.ToSlash(proot)
	if !ess.IsFileExists(proot) {
		return &os.PathError{Op: "open", Path: proot, Err: os.ErrNotExist}
	}

	if mode {
		cliLog.Infof("|-- Processing mount: '%s' <== '%s'", vroot, proot)
	}
	b, err := generateVFSSource(mode, appBaseDir, vroot, proot, skipList, noGzipList)
	if err != nil {
		return err
	}

	// destination file
	filename := fmt.Sprintf("aah%s_vfs.go", strings.Replace(vroot, "/", "_", -1))
	absFilepath := filepath.Join(appBaseDir, "app", "generated", filename)
	_ = ess.MkDirAll(filepath.Dir(absFilepath), permRWXRXRX)
	return ioutil.WriteFile(absFilepath, b, permRWXRXRX)
}

// generateVFSSource method creates Virtual FileSystem (VFS) code
// to add files and directories within binary for configured Mount points
// on file aah.project.
func generateVFSSource(mode bool, appBaseDir, vroot, proot string, skipList ess.Excludes, noGzipList []string) ([]byte, error) {
	err := skipList.Validate()
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	startTmpl := "vfs_start_embed"
	if !mode {
		startTmpl = "vfs_start_mount"
	}
	if err = vfsTmpl.ExecuteTemplate(buf, startTmpl, aah.Data{
		"Mode":         mode,
		"MountPath":    vroot,
		"PhysicalPath": proot,
	}); err != nil {
		return nil, err
	}

	// non-single binary mode, exit here
	if !mode {
		_s(fmt.Fprint(buf, "\n}"))
		return format.Source(buf.Bytes())
	}

	files := make(map[string]os.FileInfo)
	if err := ess.Walk(proot, func(fpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		fpath = filepath.ToSlash(fpath)
		// fname := path.Base(fpath)
		if skipList.Match(fpath, appBaseDir) {
			// TODO clean up later, improved in GH #245
			// if fname == "app" && (strings.Contains(fpath, "/pages/") ||
			// 	fpath == filepath.ToSlash(appBaseDir)) {
			// 	goto sc
			// }

			cliLog.Debugf("     |-- Skipping: %s", fpath)
			if info.IsDir() {
				return filepath.SkipDir // skip directory
			}
			return nil // skip file
		}
		// sc:

		if info.IsDir() {
			mp := filepath.ToSlash(filepath.Join(vroot, strings.TrimPrefix(fpath, proot)))

			if err = vfsTmpl.ExecuteTemplate(buf, "vfs_dir", aah.Data{
				"Node": &vfs.NodeInfo{Dir: info.IsDir(), Path: mp, Time: info.ModTime()},
			}); err != nil {
				return err
			}
		} else {
			files[fpath] = info
		}

		return nil
	}); err != nil {
		return nil, err
	}

	_s(fmt.Fprintf(buf, "\n// Adding files into VFS\n"))
	for fname, info := range files {
		f, err := os.Open(fname)
		if err != nil {
			logError(err)
			continue
		}

		cliLog.Debugf("     |-- Processing: %s", fname)
		mp := filepath.ToSlash(filepath.Join(vroot, strings.TrimPrefix(fname, proot)))

		if err = vfsTmpl.ExecuteTemplate(buf, "vfs_file", aah.Data{
			"Node": &vfs.NodeInfo{DataSize: info.Size(), Path: mp, Time: info.ModTime()},
		}); err != nil {
			logError(err)
			return nil, err
		}

		if info.Size() > 0 {
			if err = convertFile(buf, f, info, noGzip(noGzipList, info.Name())); err != nil {
				logError(err)
				return nil, err
			}
		}
		_s(fmt.Fprint(buf, "\"))\n\n"))
		ess.CloseQuietly(f)
	}

	_s(fmt.Fprint(buf, "}"))
	return format.Source(buf.Bytes())
}

func convertFile(buf *bytes.Buffer, r io.ReadSeeker, fi os.FileInfo, noGzip bool) error {
	restorePoint := buf.Len()
	w := &stringWriter{w: buf}

	// if its already less then MTU size or gzip not required
	if fi.Size() <= defaultGzipMinSize || noGzip {
		_, err := io.Copy(w, r)
		return err
	}

	gw := gzip.NewWriter(w)
	_, err := io.Copy(gw, r)
	if err != nil {
		return err
	}

	if err = gw.Close(); err != nil {
		return err
	}

	if int64(w.size) >= fi.Size() {
		if _, err = r.Seek(0, io.SeekStart); err != nil {
			return err
		}

		buf.Truncate(restorePoint)
		if _, err = io.Copy(w, r); err != nil {
			return err
		}
	}

	return nil
}

const lowerHex = "0123456789abcdef"

// https://github.com/go-bindata/go-bindata/blob/master/stringwriter.go
type stringWriter struct {
	w    io.Writer
	size int
}

func (s *stringWriter) Write(p []byte) (n int, err error) {
	buf := []byte(`\x00`)
	for _, b := range p {
		buf[2], buf[3] = lowerHex[b/16], lowerHex[b%16]
		if _, err = s.w.Write(buf); err != nil {
			return
		}
		n++
		s.size++
	}
	return
}

func timeStr(t time.Time) string {
	if t.IsZero() {
		return "time.Time{}"
	}
	t = t.UTC() // always go with UTC
	return fmt.Sprintf("time.Date(%d, %d, %d, %d, %d, %d, %d, time.UTC)",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond())
}

func _s(_ ...interface{}) {}

func noGzip(noGzipList []string, name string) bool {
	for _, t := range noGzipList {
		if strings.HasSuffix(name, t) {
			return true
		}
	}
	return false
}

var vfsTmplFuncMap = template.FuncMap{
	"timestr": timeStr,
}

const vfsTmplStr = `{{ define "vfs_start_mount"}} // Code generated by aah CLI - VFS, DO NOT EDIT.

package generated

import ({{ if .Mode }}
	"time"{{ end }}
	
	"aahframe.work"{{ if .Mode }}
	"aahframe.work/vfs"{{ end }}
)

func init() {
	app := aah.App()
	{{ if .Mode }}app.VFS().SetEmbeddedMode(){{ end }}

	if err := app.VFS().AddMount("{{ .MountPath }}", "{{ .PhysicalPath }}"); err != nil {
		app.Log().Fatal(err)
	}
{{ end }}

{{ define "vfs_start_embed" }}{{ template "vfs_start_mount" . }}

  // Find Mount point
  m, err := app.VFS().FindMount("{{ .MountPath }}")
  if err != nil {
		app.Log().Fatal(err)
	}

	// Adding directories into VFS
{{- end -}}

{{ define "vfs_dir" }}
	m.AddDir(&vfs.NodeInfo{
		Dir: {{ .Node.Dir }},
		Path: "{{ .Node.Path }}",
		Time: {{ .Node.Time | timestr }},
	})
{{ end }}

{{ define "vfs_file" }}
	m.AddFile(&vfs.NodeInfo{
		DataSize: {{ .Node.DataSize }},
		Path: "{{ .Node.Path }}",
		Time: {{ .Node.Time | timestr }},
	},
	[]byte("
{{- end }}
`
