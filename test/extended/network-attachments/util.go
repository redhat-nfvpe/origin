package networkattachments

import (
	"os/exec"

	testexutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	//. "github.com/onsi/gomega"
)

func implementMultus() bool {
	// We don't use testexutil.NewCLI() here because it can't be called from BeforeEach()
	out, err := exec.Command(
		"oc", "--config="+testexutil.KubeConfigPath(),
		"get", "daemonset", "--namespace=openshift-multus",
		"multus",
		"--template={{.metadata.name}}",
	).CombinedOutput()
	daemonsetName := string(out)
	if err != nil {
		e2e.Logf("Could not find multus daemonset: %v", err)
		return false
	}
	if daemonsetName == "multus" {
		return true
	}
	return false
}

func InNetworkAttachmentContext(body func()) {
	Context("when using multus", func() {
		BeforeEach(func() {
			if !implementMultus() {
				e2e.Skipf("This plugin does not implement NetworkAttachment, hence skipped.")
			}
		})

		body()
	})
}
