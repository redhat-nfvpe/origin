package sriovnetwork

import (
	"fmt"
	"os/exec"

	testexutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	Context("when multus is enabled", func() {
		BeforeEach(func() {
			if !implementMultus() {
				e2e.Skipf("This plugin does not implement NetworkAttachment, hence skipped.")
			}
		})

		body()
	})
}

func CheckSRIOVDaemonStatus(f *e2e.Framework, name string) error {
	ds, err := f.ClientSet.AppsV1().DaemonSets("openshift-sriov").Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not get %s daemon set from v1.", name)
	}
	desired, scheduled, ready := ds.Status.DesiredNumberScheduled, ds.Status.CurrentNumberScheduled, ds.Status.    NumberReady
	if desired != scheduled && desired != ready {
		return fmt.Errorf("Error in daemon status. DesiredScheduled: %d, CurrentScheduled: %d, Ready: %d",     desired, scheduled, ready)
	}
	return nil
}
