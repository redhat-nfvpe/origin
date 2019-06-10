package sriovnetwork

import (
	"fmt"
	"os/exec"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	. "github.com/onsi/ginkgo"
	//. "github.com/onsi/gomega"
)

const (
	debugPodName = "sriov-debug-pod"
)

func implementMultus() bool {
	// We don't use exutil.NewCLI() here because it can't be called from BeforeEach()
	out, err := exec.Command(
		"oc", "--config="+exutil.KubeConfigPath(),
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

func DebugListHostInt(oc *exutil.CLI) error {
	pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).
		Get(debugPodName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	out, err := oc.AsAdmin().Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name,
		"--", "ls", "/sys/class/net/").Output()
	if err != nil {
		return err
	}
	By(fmt.Sprintf("ls /sys/class/net/ output: %s ", out))
	return nil
}

func CreateDebugPod(oc *exutil.CLI) error {
	By("Creating Debug pod")
	DebugPodFixture := exutil.FixturePath("testdata", "sriovnetwork", "debug-pod.yaml")

	err := oc.AsAdmin().Run("create").Args("-f", DebugPodFixture).Execute()
	if err != nil {
		return err
	}

	By("Waiting for debug pod become ready")
	err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
		err = CheckPodStatus(oc, debugPodName)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	return err
}

func CheckPodStatus(oc *exutil.CLI, name string) error {
	pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).
		Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not get %s pod from v1.", name)
	}
	if pod.Status.Phase == corev1.PodRunning {
		return nil
	}
	return fmt.Errorf("Error in pod status. %s", pod.Status.Phase)
}

func CheckSRIOVDaemonStatus(f *e2e.Framework, namespace string, name string) error {
	ds, err := f.ClientSet.AppsV1().DaemonSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not get %s daemon set from v1.", name)
	}
	desired := ds.Status.DesiredNumberScheduled
	scheduled := ds.Status.CurrentNumberScheduled
	ready := ds.Status.NumberReady
	if desired != scheduled && desired != ready {
		return fmt.Errorf("Error in daemon status. desired: %d, scheduled: %d, ready: %d",
			desired, scheduled, ready)
	}
	return nil
}
