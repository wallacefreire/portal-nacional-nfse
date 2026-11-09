package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	backgroundFlag  = "--segundo-plano"
	runKey          = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName    = "PortalNacional"
	hkeyCurrentUser = 0x80000001
	regSZ           = 1
)

func registerAutostart() {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	if isTemporaryBuild(exe) {
		return
	}

	command := fmt.Sprintf(`"%s" %s`, exe, backgroundFlag)
	if err := setRunValue(command); err != nil {
		log.Println("Não consegui registrar o início automático com o Windows:", err)
	}
}

func waitForDrive(config Config, limit time.Duration) Config {
	deadline := time.Now().Add(limit)
	for {
		clientsCSV := resolveDriveLetter(config.ClientsCSV)
		if _, err := os.Stat(clientsCSV); err == nil || time.Now().After(deadline) {
			config.ClientsCSV = clientsCSV
			config.CertificatesDir = resolveDriveLetter(config.CertificatesDir)
			config.ConvertedDir = resolveDriveLetter(config.ConvertedDir)
			return config
		}
		time.Sleep(5 * time.Second)
	}
}

func isTemporaryBuild(exe string) bool {
	lower := strings.ToLower(exe)
	temp := strings.ToLower(filepath.Clean(os.TempDir()))
	return strings.Contains(lower, `\go-build`) || strings.HasPrefix(lower, temp)
}

func setRunValue(command string) error {
	subKey, _ := syscall.UTF16PtrFromString(runKey)
	name, _ := syscall.UTF16PtrFromString(runValueName)
	data, err := syscall.UTF16FromString(command)
	if err != nil {
		return err
	}

	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	setKeyValue := advapi32.NewProc("RegSetKeyValueW")

	status, _, _ := setKeyValue.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(subKey)),
		uintptr(unsafe.Pointer(name)),
		regSZ,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)*2),
	)
	if status != 0 {
		return syscall.Errno(status)
	}
	return nil
}
