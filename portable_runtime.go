package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ghostscriptBundle is populated during the release build, not downloaded on the end-user PC.
//
//go:embed assets/ghostscript_bundle.zip
var ghostscriptBundle []byte

func bundledPaths(dir string) (string, string, string) {
	executable := filepath.Join(dir, "bin", "gswin64c.exe")
	profile := filepath.Join(dir, "profiles", "ISOcoated_v2_eci.icc")
	parts := []string{
		filepath.Join(dir, "Resource", "Init"),
		filepath.Join(dir, "Resource", "Font"),
		filepath.Join(dir, "lib"),
		filepath.Join(dir, "fonts"),
	}
	return executable, profile, strings.Join(parts, string(os.PathListSeparator))
}

// gswin64c.exe is a small Windows launcher. Its size can be under 100 KiB.
// The substantial Ghostscript engine lives in gsdll64.dll.
func validWindowsGhostscript(executable string) bool {
	fi, err := os.Stat(executable)
	if err != nil || fi.IsDir() || fi.Size() < 1024 {
		return false
	}
	dll := filepath.Join(filepath.Dir(executable), "gsdll64.dll")
	di, err := os.Stat(dll)
	if err != nil || di.IsDir() || di.Size() < 1<<20 {
		return false
	}
	f, err := os.Open(executable)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [2]byte
	return readMZ(f, magic[:])
}

func readMZ(file *os.File, magic []byte) bool {
	_, err := io.ReadFull(file, magic)
	return err == nil && magic[0] == 'M' && magic[1] == 'Z'
}

func isBundledRuntimeReady(dir string, expectedHash string) bool {
	marker, err := os.ReadFile(filepath.Join(dir, ".ready"))
	if err != nil || strings.TrimSpace(string(marker)) != expectedHash {
		return false
	}
	exe, profile, _ := bundledPaths(dir)
	return validWindowsGhostscript(exe) && validFOGRA(profile)
}

// ensureBundledRuntime extracts the embedded Ghostscript executables and ICC data into
// a private per-user cache once. It neither runs a Windows installer nor accesses the network.
func ensureBundledRuntime() (string, string, string, error) {
	if len(ghostscriptBundle) < 1<<20 {
		return "", "", "", fmt.Errorf("релиз собран без официального Ghostscript: требуется выпуск из полного пакета ресурсов")
	}
	digest := sha256.Sum256(ghostscriptBundle)
	fingerprint := hex.EncodeToString(digest[:])
	root, err := os.UserCacheDir()
	if err != nil {
		root, err = os.UserConfigDir()
	}
	if err != nil {
		return "", "", "", err
	}
	base := filepath.Join(root, "PDF_Color_Checker", "runtime")
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", "", "", err
	}
	target := filepath.Join(base, fingerprint[:20])
	if isBundledRuntimeReady(target, fingerprint) {
		gs, icc, lib := bundledPaths(target)
		return gs, icc, lib, nil
	}
	lockDir := target + ".lock"
	acquired := false
	for i := 0; i < 200; i++ {
		if e := os.Mkdir(lockDir, 0700); e == nil {
			acquired = true
			break
		}
		if isBundledRuntimeReady(target, fingerprint) {
			gs, icc, lib := bundledPaths(target)
			return gs, icc, lib, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !acquired {
		return "", "", "", fmt.Errorf("не удалось подготовить движок: ресурс занят другим экземпляром программы")
	}
	defer os.Remove(lockDir)
	temp, err := os.MkdirTemp(base, "unpack-")
	if err != nil {
		return "", "", "", err
	}
	defer os.RemoveAll(temp)
	zr, err := zip.NewReader(bytes.NewReader(ghostscriptBundle), int64(len(ghostscriptBundle)))
	if err != nil {
		return "", "", "", fmt.Errorf("встроенный архив повреждён: %w", err)
	}
	var totalBytes int64
	for _, entry := range zr.File {
		if entry.Mode()&os.ModeSymlink != 0 {
			return "", "", "", fmt.Errorf("недопустимая ссылка в архиве")
		}
		clean := filepath.Clean(filepath.FromSlash(entry.Name))
		if clean == "." {
			continue
		}
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || strings.Contains(clean, ":") {
			return "", "", "", fmt.Errorf("недопустимый путь в архиве: %s", entry.Name)
		}
		dest := filepath.Join(temp, clean)
		if entry.FileInfo().IsDir() {
			if err = os.MkdirAll(dest, 0700); err != nil {
				return "", "", "", err
			}
			continue
		}
		if entry.UncompressedSize64 > 256<<20 {
			return "", "", "", fmt.Errorf("ресурс слишком велик: %s", entry.Name)
		}
		totalBytes += int64(entry.UncompressedSize64)
		if totalBytes > 750<<20 {
			return "", "", "", fmt.Errorf("ресурсы превышают допустимый размер")
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return "", "", "", err
		}
		input, e := entry.Open()
		if e != nil {
			return "", "", "", e
		}
		output, e := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			input.Close()
			return "", "", "", e
		}
		_, e = io.Copy(output, io.LimitReader(input, int64(entry.UncompressedSize64)+1))
		output.Close()
		input.Close()
		if e != nil {
			return "", "", "", e
		}
	}
	gs, icc, _ := bundledPaths(temp)
	if !validWindowsGhostscript(gs) {
		return "", "", "", fmt.Errorf("встроенный движок Ghostscript неполный: требуются bin/gswin64c.exe и bin/gsdll64.dll")
	}
	if !validFOGRA(icc) {
		return "", "", "", fmt.Errorf("встроенный ICC-профиль FOGRA39 отсутствует или повреждён")
	}
	if e := os.WriteFile(filepath.Join(temp, ".ready"), []byte(fingerprint), 0600); e != nil {
		return "", "", "", e
	}
	if _, e := os.Stat(target); e == nil {
		_ = os.RemoveAll(target)
	}
	if e := os.Rename(temp, target); e != nil {
		return "", "", "", e
	}
	gs, icc, lib := bundledPaths(target)
	return gs, icc, lib, nil
}
