package executable

import (
	"io/ioutil"
	"os/exec"

	"github.com/cilium-team/docker-collector/Godeps/_workspace/src/github.com/op/go-logging"
)

var (
	log = logging.MustGetLogger("docker-collector")
)

// This way it's easier to mock this func on tests.
var ExecShCommand = execShCmd

// execShCmd executes the given strCmd on the with the help of /bin/sh.
func execShCmd(strCmd string) ([]byte, error) {
	log.Debug("Executing %+v", strCmd)
	cmd := exec.Command("/bin/sh", "-c", strCmd)

	stdoutpipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("Error stdout: %s. for command: %s", err, strCmd)
		return nil, err
	}
	stderrpipe, err := cmd.StderrPipe()
	if err != nil {
		log.Error("Error stderr: %s. for command: %s", err, strCmd)
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		log.Error("Error: %s. for command: %s", err, strCmd)
		return nil, err
	}
	stdout, errstderr := ioutil.ReadAll(stdoutpipe)
	stderr, errstdout := ioutil.ReadAll(stderrpipe)

	cmderr := cmd.Wait()

	if errstderr != nil {
		log.Debug("Stdout err: %v", errstderr)
	}
	if errstdout != nil {
		log.Debug("Stderr err: %v", errstdout)
	}
	log.Debug("Stdout is: '%s'\n", stdout)
	log.Debug("Stderr is: '%s'\n", stderr)
	if cmderr != nil {
		log.Error("cmderr: %v, %v", cmderr, string(stderr))
	}
	return stdout, cmderr
}
