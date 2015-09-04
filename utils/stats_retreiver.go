package utils

import (
	"errors"
	"strconv"
	"strings"

	ue "github.com/cilium-team/docker-collector/utils/executable"

	"github.com/cilium-team/docker-collector/Godeps/_workspace/src/github.com/op/go-logging"
)

const procPath = "/proc"

var log = logging.MustGetLogger("docker-collector")

func ReadContainerFromProc(pid int, fullpath string) (string, error) {
	b, err := ue.ExecShCommand(`cat ` + procPath + `/` + strconv.Itoa(pid) + `/root` + fullpath)
	if err != nil {
		return "", err
	}
	bstr := strings.TrimRight(string(b), "\n")
	if strings.Contains(bstr, "Cannot run exec command") ||
		strings.Contains(bstr, "No such exec") ||
		strings.Contains(bstr, "Failed") ||
		strings.Contains(bstr, "invalid argument") ||
		bstr == "" {
		return "", errors.New("Invalid path")
	}
	return bstr, nil
}
