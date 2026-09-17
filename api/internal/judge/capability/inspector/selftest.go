package inspector

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func SandboxSelfTest() (SandboxReport, error) {
	report := SandboxReport{NonRootUser: os.Geteuid() == 65532}
	uidMap, uidErr := os.ReadFile("/proc/self/uid_map")
	report.UserNamespace = uidErr == nil && hasIsolatedUserNamespace(uidMap)

	rootProbe := "/.judgify-capability-write-test"
	rootErr := os.WriteFile(rootProbe, []byte("denied"), 0o600)
	report.ReadOnlyRoot = rootErr != nil
	if rootErr == nil {
		_ = os.Remove(rootProbe)
	}

	interfaces, networkErr := os.ReadDir("/sys/class/net")
	report.NetworkDisabled = networkErr == nil && len(interfaces) == 1 && interfaces[0].Name() == "lo"
	status, statusErr := os.ReadFile("/proc/self/status")
	if statusErr == nil {
		values := procStatusValues(status)
		report.NoNewPrivileges = values["NoNewPrivs"] == "1"
		report.Seccomp = values["Seccomp"] != "" && values["Seccomp"] != "0"
		report.CapabilitiesDropped = strings.TrimLeft(values["CapEff"], "0") == ""
	}

	cgroup, cgroupErr := os.ReadFile("/proc/self/cgroup")
	cgroupDirectory, validCgroup := selfCgroupV2Directory(cgroup)
	if cgroupErr == nil && validCgroup {
		report.CPUAccounting = readable(filepath.Join(cgroupDirectory, "cpu.stat"))
		report.MemoryAccounting = readable(filepath.Join(cgroupDirectory, "memory.current"))
		report.OOMAccounting = containsField(filepath.Join(cgroupDirectory, "memory.events"), "oom")
		pidsMaximum, err := os.ReadFile(filepath.Join(cgroupDirectory, "pids.max"))
		report.PIDEnforcement = err == nil && strings.TrimSpace(string(pidsMaximum)) != "" &&
			strings.TrimSpace(string(pidsMaximum)) != "max"
	}

	if err := os.WriteFile(archiveProofPath, []byte("judgify capability archive proof\n"), 0o400); err != nil {
		return SandboxReport{}, errors.Join(ErrInspectionDenied, err)
	}
	if !report.NonRootUser || !report.UserNamespace || !report.ReadOnlyRoot || !report.NetworkDisabled ||
		!report.NoNewPrivileges || !report.Seccomp || !report.CapabilitiesDropped ||
		!report.CPUAccounting || !report.MemoryAccounting || !report.PIDEnforcement || !report.OOMAccounting {
		return SandboxReport{}, ErrInspectionDenied
	}
	return report, nil
}

func hasIsolatedUserNamespace(data []byte) bool {
	fields := strings.Fields(string(data))
	if len(fields) < 3 || len(fields)%3 != 0 {
		return false
	}
	return fields[0] != "0" || fields[1] != "0" || fields[2] != "4294967295"
}

func procStatusValues(data []byte) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, ":")
		if found {
			values[key] = strings.TrimSpace(value)
		}
	}
	return values
}

func selfCgroupV2Directory(data []byte) (string, bool) {
	membership := strings.TrimSuffix(string(data), "\n")
	parts := strings.Split(membership, ":")
	if len(parts) != 3 || parts[0] != "0" || parts[1] != "" ||
		!filepath.IsAbs(parts[2]) || filepath.Clean(parts[2]) != parts[2] {
		return "", false
	}
	return filepath.Join("/sys/fs/cgroup", parts[2]), true
}

func readable(name string) bool {
	_, err := os.ReadFile(name)
	return err == nil
}

func containsField(name string, field string) bool {
	data, err := os.ReadFile(name)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == field {
			return true
		}
	}
	return false
}
