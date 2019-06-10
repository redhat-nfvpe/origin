package sriovnetwork

import (
	"fmt"
	"strings"
	"time"

        exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[Area:Networking] SRIOV Network Device Plugin", func() {
	defer GinkgoRecover()

	InNetworkAttachmentContext(func() {
		oc := exutil.NewCLI("sriov", exutil.KubeConfigPath())
		f1 := oc.KubeFramework()

		It("should successfully create/delete SRIOV device plugin daemonsets", func() {
			DevicePluginDaemonFixture := exutil.FixturePath("testdata", "sriovnetwork", "dp-daemon.yaml")
			By("Creating SRIOV device plugin daemonset")
			err := oc.AsAdmin().Run("create").Args("-f", DevicePluginDaemonFixture).Execute()
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for SRIOV daemonsets become ready")
			err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
				err = CheckSRIOVDaemonStatus(f1, oc.Namespace(), "sriov-device-plugin")
				if err != nil {
					return false, nil
				}
				return true, nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Deleting SRIOV device plugin daemonset")
			err = oc.AsAdmin().Run("delete").Args("-f", DevicePluginDaemonFixture).Execute()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully create SRIOV VFs", func() {
			DevicePluginDaemonFixture := exutil.FixturePath("testdata",
				"sriovnetwork", "dp-daemon.yaml")

			err := CreateDebugPod(oc)
			Expect(err).NotTo(HaveOccurred())

			err = DebugListHostInt(oc)
			Expect(err).NotTo(HaveOccurred())

			sriovNodes := make([]string, 0)
			options := metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker="}
			workerNodes, _ := f1.ClientSet.CoreV1().Nodes().List(options)

			pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).
				Get(debugPodName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			for _, n := range workerNodes.Items {
				out, err := oc.AsAdmin().Run("exec").Args(pod.Name,
					"-c", pod.Spec.Containers[0].Name,
					"--", "/provision_sriov.sh", "-c", "2").Output()

				Expect(err).NotTo(HaveOccurred())
				By(fmt.Sprintf("provision_sriov.sh output: %s ", out))

				if strings.Contains(out, "successfully configured") {
					sriovNodes = append(sriovNodes, n.GetName())
				} else if strings.Contains(out, "failed to configure") {
					e2e.Failf("Unable to provision SR-IOV VFs on node %s", n.GetName())
				} else {
					e2e.Skipf("Skipping node %s as it doesn't contain SR-IOV capable NIC.",
						n.GetName())
				}
			}

			if len(sriovNodes) > 0 {
				By("Creating SRIOV device plugin daemonset")
				err = oc.AsAdmin().Run("create").Args("-f", DevicePluginDaemonFixture).Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV daemonsets become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckSRIOVDaemonStatus(f1, oc.Namespace(), "sriov-device-plugin")
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())
			}

			for _, n := range sriovNodes {
				out, err := oc.AsAdmin().Run("get").Args("node", n).
					Template("{{ .status.allocatable }}").Output()
				Expect(err).NotTo(HaveOccurred())
				By(fmt.Sprintf("Node %s allocatable output: %s", n, out))
			}
		})
	})
})
