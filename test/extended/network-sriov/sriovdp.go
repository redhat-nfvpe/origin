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

			By("Creating SRIOV device plugin config map")
			err := oc.AsAdmin().Run("create").Args("-f", DevicePluginConfigFixture).Execute()
			Expect(err).NotTo(HaveOccurred())

			By("Creating SRIOV device plugin daemonset")
			err = oc.AsAdmin().Run("create").Args("-f", DevicePluginDaemonFixture).Execute()
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for SRIOV daemonsets become ready")
			err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
				err = CheckSRIOVDaemonStatus(f1, oc.Namespace(), sriovDPPodName)
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

		It("should report correct SRIOV VF numbers", func() {

			By("Creating SRIOV debug pod")
			err := CreateDebugPod(oc)
			Expect(err).NotTo(HaveOccurred())

			By("Debug list host interfaces")
			err = DebugListHostInt(oc)
			Expect(err).NotTo(HaveOccurred())

			By("Get all worker nodes")
			options := metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker="}
			workerNodes, _ := f1.ClientSet.CoreV1().Nodes().List(options)

			pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).
				Get(debugPodName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			resConfList := ResourceConfList{}
			nicMatrix := InitNICMatrix()

			By("Provision SR-IOV on worker nodes")
			for _, n := range workerNodes.Items {
				for _, dev := range nicMatrix.NICs {
					out, err := oc.AsAdmin().Run("exec").Args(pod.Name,
						"-c", pod.Spec.Containers[0].Name,
						"--", "/provision_sriov.sh", "-c", sriovNumVFs,
						"-v", dev.VendorID, "-d", dev.DeviceID).Output()

					Expect(err).NotTo(HaveOccurred())
					By(fmt.Sprintf("provision_sriov.sh output: %s ", out))

					if strings.Contains(out, "successfully configured") {
						resConfList.ResourceList = append(resConfList.ResourceList,
							ResourceConfig{
							NodeName: n.GetName(),
							ResourceNum: sriovNumVFs,
							ResourceName: dev.ResourceName})
					} else if strings.Contains(out, "failed to configure") {
						e2e.Failf("Unable to provision SR-IOV VFs on node %s", n.GetName())
					} else {
						e2e.Logf("Skipping node %s.", n.GetName())
					}
				}
			}

			if len(resConfList.ResourceList) > 0 {
				By("Creating SRIOV device plugin config map")
				err = oc.AsAdmin().Run("create").Args("-f", DevicePluginConfigFixture).Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Creating SRIOV device plugin daemonset")
				err = oc.AsAdmin().Run("create").Args("-f", DevicePluginDaemonFixture).Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV daemonsets become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckSRIOVDaemonStatus(f1, oc.Namespace(), sriovDPPodName)
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())
			} else {
				e2e.Skipf("Skipping, no SR-IOV capable NIC configured.")
			}

			for _, n := range resConfList.ResourceList {
				out, err := oc.AsAdmin().Run("get").Args("node", n.NodeName).
					Template("{{ .status.allocatable }}").Output()
				Expect(err).NotTo(HaveOccurred())
				By(fmt.Sprintf("Node %s allocatable output: %s", n.NodeName, out))
			}
		})
	})
})
