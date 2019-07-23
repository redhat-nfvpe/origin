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
			err := oc.AsAdmin().Run("create").
				Args("-f", DevicePluginConfigFixture, "-n", oc.Namespace()).Execute()
			Expect(err).NotTo(HaveOccurred())

			By("Creating SRIOV device plugin daemonset")
			err = oc.AsAdmin().Run("create").
				Args("-f", DevicePluginDaemonFixture, "-n", oc.Namespace()).Execute()
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

			By("Get all worker nodes")
			options := metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker="}
			workerNodes, _ := f1.ClientSet.CoreV1().Nodes().List(options)

			resConfList := ResourceConfList{}
			nicMatrix := InitNICMatrix()

			By("Provision SR-IOV on worker nodes")
			for _, n := range workerNodes.Items {

				err := oc.AsAdmin().Run("label").
					Args("node", n.GetName(), "node.sriovStatus=provisioning").Execute()
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Creating SRIOV debug pod on Node %s", n.GetName()))
				err = CreateDebugPod(oc)
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Debug list host interfaces on Node %s", n.GetName()))
				err = DebugListHostInt(oc)
				Expect(err).NotTo(HaveOccurred())

				pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).
					Get(debugPodName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

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

				By(fmt.Sprintf("Deleting SRIOV debug pod on Node %s", n.GetName()))
				err = DeleteDebugPod(oc)
				Expect(err).NotTo(HaveOccurred())

				err = oc.AsAdmin().Run("label").
					Args("node", n.GetName(), "node.sriovStatus-").Execute()
				Expect(err).NotTo(HaveOccurred())
			}

			if len(resConfList.ResourceList) > 0 {
				for _, dev := range nicMatrix.NICs {
					By("Creating SR-IOV CRDs")
					err := oc.AsAdmin().Run("create").
						Args("-f", fmt.Sprintf("%s/crd-%s.yaml",
						TestDataFixture, dev.ResourceName)).Execute()
					Expect(err).NotTo(HaveOccurred())
				}
				By("Creating SRIOV device plugin config map")
				err := oc.AsAdmin().Run("create").
					Args("-f", DevicePluginConfigFixture, "-n", "kube-system").Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Creating SRIOV device plugin daemonset")
				err = oc.AsAdmin().Run("create").
					Args("-f", DevicePluginDaemonFixture, "-n", "kube-system").Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV Device Plugin daemonsets become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckSRIOVDaemonStatus(f1, "kube-system", sriovDPPodName)
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())

				By("Creating SRIOV CNI plugin daemonset")
				err = oc.AsAdmin().Run("create").
					Args("-f", CNIDaemonFixture, "-n", "kube-system").Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV CNI daemonsets become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckSRIOVDaemonStatus(f1, "kube-system", sriovCNIPodName)
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())
			} else {
				e2e.Skipf("Skipping, no SR-IOV capable NIC configured.")
			}

                        defer func() {
                                if len(resConfList.ResourceList) > 0 {
					for _, dev := range nicMatrix.NICs {
						By("Deleting SR-IOV CRDs")
						err := oc.AsAdmin().Run("delete").
							Args("-f", fmt.Sprintf("%s/crd-%s.yaml",
							TestDataFixture, dev.ResourceName)).Execute()
						Expect(err).NotTo(HaveOccurred())
					}
                                        By("Deleting SRIOV device plugin daemonset")
                                        err := oc.AsAdmin().Run("delete").
                                                Args("-f", DevicePluginDaemonFixture, "-n", "kube-system").
						Execute()
                                        Expect(err).NotTo(HaveOccurred())

                                        By("Deleting SRIOV device plugin config map")
                                        err = oc.AsAdmin().Run("delete").
                                                Args("-f", DevicePluginConfigFixture, "-n", "kube-system").
						Execute()
                                        Expect(err).NotTo(HaveOccurred())

                                        By("Deleting SRIOV CNI daemonset")
                                        err = oc.AsAdmin().Run("delete").
                                                Args("-f", CNIDaemonFixture, "-n", "kube-system").
						Execute()
                                        Expect(err).NotTo(HaveOccurred())
                                }
                        }()

			time.Sleep(1 * time.Minute)
			for _, n := range resConfList.ResourceList {
				templateArgs := fmt.Sprintf(
					"'{{ index .status.allocatable \"openshift.com/%s\" }}'",
					n.ResourceName)
				out, err := oc.AsAdmin().Run("get").Args("node", n.NodeName).
					Template(templateArgs).Output()
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(Equal(fmt.Sprintf("'%s'", n.ResourceNum)))
				By(fmt.Sprintf("Node %s allocatable output: %s", n.NodeName, out))
			}


			for _, n := range resConfList.ResourceList {
				By("Creating SRIOV Test Pod")
				err := oc.AsAdmin().Run("create").
					Args("-f", fmt.Sprintf("%s/pod-%s.yaml",
					TestDataFixture, n.ResourceName)).Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for testpod become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckPodStatus(oc, fmt.Sprintf("testpod-%s", n.ResourceName))
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())

				out, err := oc.AsAdmin().Run("exec").
					Args("-p", fmt.Sprintf("testpod-%s", n.ResourceName),
					"--", "/bin/bash", "-c", "ip link show dev net1").Output()
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(ContainSubstring(fmt.Sprintf("does not exist")))
				Expect(out).To(ContainSubstring(fmt.Sprintf("mtu")))
				By(fmt.Sprintf("Pod net1 output: %s", out))
			}

			defer func() {
				for _, n := range resConfList.ResourceList {
					oc.AsAdmin().Run("delete").Args("-f", fmt.Sprintf("%s/pod-%s.yaml",
						TestDataFixture, n.ResourceName)).Execute()
				}
			}()
		})
	})
})
