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

var _ = Describe("[Area:Networking][Serial] SRIOV DPDK", func() {
	defer GinkgoRecover()

	InNetworkAttachmentContext(func() {
		oc := exutil.NewCLI("sriov", exutil.KubeConfigPath())
		f1 := oc.KubeFramework()

		It("should report correct dpdk interface number", func() {

			By("Get all worker nodes")
			options := metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker="}
			workerNodes, _ := f1.ClientSet.CoreV1().Nodes().List(options)
			options = metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="}
			masterNodes, _ := f1.ClientSet.CoreV1().Nodes().List(options)

			resConfList := ResourceConfList{}
			nicMatrix := InitNICMatrix()

			var isMaster bool

			By("Provision SR-IOV and Bind to vfio-pci driver on worker nodes")
			for _, n := range workerNodes.Items {

				isMaster = false
				for _, m := range masterNodes.Items {
					if n.GetName() == m.GetName() {
						e2e.Logf("Skipping master node %s.", n.GetName())
						isMaster = true
						break
					}
				}

				if isMaster {
					continue
				}
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

				// Wait for 20s for VF to appear in debug pod
				time.Sleep(20 * time.Second)
				out, err := oc.AsAdmin().Run("exec").Args(pod.Name,
					"-c", pod.Spec.Containers[0].Name,
					"--", "/bind_dpdk.sh", "-b").Output()

				Expect(err).NotTo(HaveOccurred())
				By(fmt.Sprintf("bind_dpdk.sh output: %s ", out))

				if strings.Contains(out, "successfully bind") {
					e2e.Logf("Binding succeeded on node %s.", n.GetName())
				} else if strings.Contains(out, "failed to bind") {
					e2e.Failf("Unable to bind SR-IOV VFs to vfio on node %s",
						n.GetName())
				} else {
					e2e.Logf("Skipping dpdk bind on node %s.", n.GetName())
				}

				By(fmt.Sprintf("Deleting SRIOV debug pod on Node %s", n.GetName()))
				err = DeleteDebugPod(oc)
				Expect(err).NotTo(HaveOccurred())

				err = oc.AsAdmin().Run("label").
					Args("node", n.GetName(), "node.sriovStatus-").Execute()
				Expect(err).NotTo(HaveOccurred())
			}

			defer func() {
				By("Provision SR-IOV and unBind VFs from vfio-pci driver on worker nodes")
				for _, n := range workerNodes.Items {

					By(fmt.Sprintf("Creating SRIOV debug pod on Node %s", n.GetName()))
					err := CreateDebugPod(oc)
					Expect(err).NotTo(HaveOccurred())

					pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).
						Get(debugPodName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())

					out, err := oc.AsAdmin().Run("exec").Args(pod.Name,
						"-c", pod.Spec.Containers[0].Name,
						"--", "/bind_dpdk.sh", "-u").Output()

					Expect(err).NotTo(HaveOccurred())
					By(fmt.Sprintf("bind_dpdk.sh output: %s ", out))

					if strings.Contains(out, "successfully unbind") {
						e2e.Logf("unBind succeed on node %s", n.GetName())
					} else if strings.Contains(out, "failed to unbind") {
						e2e.Failf("Unbind VFs failed on node %s",
						n.GetName())
					} else {
						e2e.Logf("Skip unbind on node %s.", n.GetName())
					}

					By(fmt.Sprintf("Deleting SRIOV debug pod on Node %s", n.GetName()))
					err = DeleteDebugPod(oc)
					Expect(err).NotTo(HaveOccurred())
				}
			}()

			if len(resConfList.ResourceList) > 0 {
				for _, dev := range nicMatrix.NICs {
					By("Creating SR-IOV DPDK CRDs")
					err := oc.AsAdmin().Run("create").
						Args("-f", fmt.Sprintf("%s/crd-%s-dpdk.yaml",
						DPDKTestDataFixture, dev.ResourceName)).Execute()
					Expect(err).NotTo(HaveOccurred())
				}
				By("Creating SRIOV device plugin config map")
				err := oc.AsAdmin().Run("create").Args("-f",
					fmt.Sprintf("%s/%s", DPDKTestDataFixture, sriovDPConfigMap),
					"-n", "kube-system").Execute()
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
							Args("-f", fmt.Sprintf("%s/crd-%s-dpdk.yaml",
							DPDKTestDataFixture, dev.ResourceName)).Execute()
						Expect(err).NotTo(HaveOccurred())
					}
                                        By("Deleting SRIOV device plugin daemonset")
                                        err := oc.AsAdmin().Run("delete").
                                                Args("-f", DevicePluginDaemonFixture, "-n", "kube-system").
						Execute()
                                        Expect(err).NotTo(HaveOccurred())

                                        By("Deleting SRIOV device plugin config map")
                                        err = oc.AsAdmin().Run("delete").Args("-f",
						fmt.Sprintf("%s/%s", DPDKTestDataFixture, sriovDPConfigMap),
						"-n", "kube-system").Execute()
                                        Expect(err).NotTo(HaveOccurred())

                                        By("Deleting SRIOV CNI daemonset")
                                        err = oc.AsAdmin().Run("delete").
                                                Args("-f", CNIDaemonFixture, "-n", "kube-system").
						Execute()
                                        Expect(err).NotTo(HaveOccurred())
                                }
                        }()

			time.Sleep(20 * time.Second)
			for _, n := range resConfList.ResourceList {
				var templateArgs string
				if n.ResourceName == "intelxxv710" {
					templateArgs = fmt.Sprintf(
						"'{{ index .status.allocatable \"openshift.com/%sdpdk\" }}'",
						n.ResourceName)
				} else {
					templateArgs = fmt.Sprintf(
						"'{{ index .status.allocatable \"openshift.com/%s\" }}'",
						n.ResourceName)
				}
				out, err := oc.AsAdmin().Run("get").Args("node", n.NodeName).
					Template(templateArgs).Output()
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(Equal(fmt.Sprintf("'%s'", n.ResourceNum)))
				By(fmt.Sprintf("Node %s allocatable output: %s", n.NodeName, out))
			}


			for _, n := range resConfList.ResourceList {
				By("Creating SRIOV DPDK Test Pod")
				err := oc.AsAdmin().Run("create").
					Args("-f", fmt.Sprintf("%s/pod-%s-dpdk.yaml",
					DPDKTestDataFixture, n.ResourceName)).Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for testpod become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckPodStatus(oc, fmt.Sprintf("testpod-%s-dpdk", n.ResourceName))
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())

				if n.ResourceName == "intelxxv710" {
					out, err := oc.AsAdmin().Run("exec").
						Args(fmt.Sprintf("testpod-%s-dpdk", n.ResourceName),
						"--", "/bin/bash", "-c", "ls /dev/vfio/vfio").Output()
					Expect(err).NotTo(HaveOccurred())
					Expect(out).NotTo(ContainSubstring(fmt.Sprintf("No such file")))
					Expect(out).To(ContainSubstring(fmt.Sprintf("/dev/vfio/vfio")))

					out, err = oc.AsAdmin().Run("exec").
						Args(fmt.Sprintf("testpod-%s-dpdk", n.ResourceName),
						"--", "/bin/bash", "-c", "ls /dev/vfio").Output()
					Expect(err).NotTo(HaveOccurred())
					Expect(out).NotTo(ContainSubstring(fmt.Sprintf("No such file")))
					Expect(out).To(ContainSubstring(fmt.Sprintf("vfio")))
					outList := strings.Split(out, " ")
					for index, o := range outList {
						if index == 0 {
							Expect(o).To(ContainSubstring(fmt.Sprintf("vfio")))
						} else {
							Expect(o).Should(BeNumerically(">", 1))
						}
					}
					By(fmt.Sprintf("Pod vfio device output: %s", out))
				} else {
					out, err := oc.AsAdmin().Run("exec").
						Args(fmt.Sprintf("testpod-%s-dpdk", n.ResourceName),
						"--", "/bin/bash", "-c", "ip link show dev net1").Output()
					Expect(err).NotTo(HaveOccurred())
					Expect(out).NotTo(ContainSubstring(fmt.Sprintf("does not exist")))
					Expect(out).To(ContainSubstring(fmt.Sprintf("mtu")))
					By(fmt.Sprintf("Pod net1 output: %s", out))
				}
				oc.AsAdmin().Run("delete").Args("-f", fmt.Sprintf("%s/pod-%s-dpdk.yaml",
					DPDKTestDataFixture, n.ResourceName)).Execute()
			}
		})
	})
})
