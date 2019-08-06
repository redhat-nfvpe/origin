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
	debugPodName     = "sriov-debug-pod"
	debugPodSpec     = "debug-pod.yaml"
	sriovDPPodName   = "sriov-device-plugin"
	sriovCNIPodName  = "sriov-cni"
	sriovDPSpec      = "dp-daemon.yaml"
	sriovCNISpec     = "cni-daemon.yaml"
	sriovDPConfigMap = "config-map.yaml"
	sriovNumVFs      = "4"
	sriovAcSrvName   = "network-resources-injector"
	sriovAcSAName    = "network-resources-injector-sa"
	sriovACSrvSpec    = "admission-controller.yaml"
	sriovACAuthSpec   = "admission-controller-service-account.yaml"
)

var (
	TestDataFixture = exutil.FixturePath("testdata", "sriovnetwork")
	SRIOVTestDataFixture = exutil.FixturePath("testdata", "sriovnetwork", "sriov")
	DPDKTestDataFixture = exutil.FixturePath("testdata", "sriovnetwork", "dpdk")
	RDMATestDataFixture = exutil.FixturePath("testdata", "sriovnetwork", "rdma")
	DebugPodFixture = exutil.FixturePath("testdata", "sriovnetwork", debugPodSpec)
	CNIDaemonFixture = exutil.FixturePath("testdata", "sriovnetwork", sriovCNISpec)
	DevicePluginDaemonFixture = exutil.FixturePath("testdata", "sriovnetwork", sriovDPSpec)
	AdmissionControllerServerDaemonFixture = exutil.FixturePath("testdata", "sriovnetwork", sriovACSrvSpec)
	AdmissionControllerAuthDaemonFixture = exutil.FixturePath("testdata", "sriovnetwork", sriovACAuthSpec)
)

type ResourceConfig struct {
	NodeName	string	`json:"nodeName"`
	ResourceName	string	`json:"resourceName"`
	ResourceNum	string	`json:"resourceNum"`
}

// ResourceConfList is a list of ResourceConfig
type ResourceConfList struct {
	ResourceList []ResourceConfig `json:"resourceConfig"`
}

type Matrix struct {
	VendorID	string	`json:"vendorid"`
	DeviceID	string	`json:"deviceid"`
	ResourceName	string	`json:"resourceName"`
}

type NICMatrix struct {
	NICs	[]Matrix	`json:"matrix"`
}

func InitNICMatrix() *NICMatrix {
	return &NICMatrix{
		NICs: []Matrix{
			{VendorID: "0x8086", DeviceID: "0x158b", ResourceName: "intelxxv710"},
			{VendorID: "0x15b3", DeviceID: "0x1015", ResourceName: "mlx4lx"},
			{VendorID: "0x15b3", DeviceID: "0x1017", ResourceName: "mlx5"},
		},
	}
}

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

func DeleteDebugPod(oc *exutil.CLI) error {
	By("Deleting Debug pod")
	err := oc.AsAdmin().Run("delete").Args("-f", DebugPodFixture).Execute()
	if err != nil {
		return err
	}
	return nil
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

func CheckServiceAccountStatus(oc *exutil.CLI, namespace string, name string) error {
	sa, err := oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace).
		Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not get %s service account from v1.", name)
	}
	if sa == nil {
		return fmt.Errorf("Could not find %s service account from v1.", name)
	}
	return nil
}
