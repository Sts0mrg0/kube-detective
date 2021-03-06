package detective

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/1.5/pkg/api/v1"
)

func isNodeConditionSetAsExpected(node *v1.Node, conditionType v1.NodeConditionType, wantTrue bool) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == conditionType {
			if (cond.Status == v1.ConditionTrue) == wantTrue {
				return true
			} else {
				return false
			}
		}
	}
	return false
}

func filterNodes(nodeList *v1.NodeList, fn func(node v1.Node) bool) {
	var l []v1.Node

	for _, node := range nodeList.Items {
		if fn(node) {
			l = append(l, node)
		}
	}
	nodeList.Items = l
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func RunHostCmd(ns, name, cmd string) (string, error) {
	return RunKubectl("exec", fmt.Sprintf("--namespace=%v", ns), name, "--", "/bin/sh", "-c", cmd)
}

func RunKubectl(args ...string) (string, error) {
	return NewKubectlCommand(args...).Exec()
}

func NewKubectlCommand(args ...string) *kubectlBuilder {
	b := new(kubectlBuilder)
	b.cmd = KubectlCmd(args...)
	return b
}

func KubectlCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("monsoonctl", args...)
	return cmd
}

type kubectlBuilder struct {
	cmd     *exec.Cmd
	timeout <-chan time.Time
}

func (b kubectlBuilder) Exec() (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := b.cmd
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	glog.V(4).Infof("Running '%s %s'", cmd.Path, strings.Join(cmd.Args[1:], " ")) // skip arg[0] as it is printed separately
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("Error starting %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v\n", cmd, cmd.Stdout, cmd.Stderr, err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
	}()
	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("Error running %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v\n", cmd, cmd.Stdout, cmd.Stderr, err)
		}
	case <-b.timeout:
		b.cmd.Process.Kill()
		return "", fmt.Errorf("Timed out waiting for command %v:\nCommand stdout:\n%v\nstderr:\n%v\n", cmd, cmd.Stdout, cmd.Stderr)
	}
	glog.V(4).Infof("stderr: %q", stderr.String())
	return stdout.String(), nil
}
