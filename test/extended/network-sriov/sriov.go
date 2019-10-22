package sriovnetwork

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[Area:Networking][Serial] SRIOV", func() {
	defer GinkgoRecover()

	InNetworkAttachmentContext(func() {
		oc := exutil.NewCLI("sriov", exutil.KubeConfigPath())
		f1 := oc.KubeFramework()

		It("should report correct sriov VF numbers", func() {

			By("Get all worker nodes")
			options := metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker="}
			workerNodes, _ := f1.ClientSet.CoreV1().Nodes().List(options)
			options = metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="}
			masterNodes, _ := f1.ClientSet.CoreV1().Nodes().List(options)

			resConfList := ResourceConfList{}
			nicMatrix := InitNICMatrix()

			var isMaster bool

			By("Provision SR-IOV on worker nodes")
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

				defer func() {
					err = oc.AsAdmin().Run("label").
						Args("node", n.GetName(), "node.sriovStatus-").Execute()
					Expect(err).NotTo(HaveOccurred())
				}()

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

			}

			if len(resConfList.ResourceList) > 0 {
				By("Creating Admission Controller Service")
				err := oc.AsAdmin().Run("create").
					Args("-f", AdmissionControllerSvcDaemonFixture, "-n", "kube-system").Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV Admission Controller Service to become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckServiceStatus(oc, "kube-system", sriovAcSvcName)
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())

				By("Creating Admission Controller Service Account")
				err = oc.AsAdmin().Run("create").
					Args("-f", AdmissionControllerSvcAcctDaemonFixture, "-n", "kube-system").Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV Admission Controller Service Account to become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckServiceAccountStatus(oc, "kube-system", sriovAcSvcAcctName)
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())

				By("Creating Admission Controller ConfigMap")
				err = oc.AsAdmin().Run("create").
					Args("-f", AdmissionControllerConfigMapFixture, "-n", "kube-system").Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV Admission Controller ConfigMap to become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckConfigMapStatus(oc, "kube-system", sriovAcConfigMapName)
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())

				By("Creating SRIOV Admission Controller Webhook")
				_, err = exec.Command("sh", "-c",
					"cat /home/slave1/workspace/OCP-SRIOV-E2E/origin/test/extended/testdata/sriovnetwork/sriov-admission-controller-webhook.yaml" +
					"| /home/slave1/workspace/OCP-SRIOV-E2E/origin/test/extended/testdata/sriovnetwork/sriov-webhook-patch-bundle.sh" +
					"| oc create -f - -n kube-system").CombinedOutput()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV Admission Webhook to become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckWebhookStatus(oc, sriovAcWebhookName)
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())

				By("Creating Admission Controller Server")
				err = oc.AsAdmin().Run("create").
					Args("-f", AdmissionControllerServerDaemonFixture, "-n", "kube-system").Execute()
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for SRIOV Admission Controller Server to become ready")
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					err = CheckSRIOVDaemonStatus(f1, "kube-system", sriovAcSrvName)
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())

				for _, dev := range nicMatrix.NICs {
					By("Creating SR-IOV CRDs")
					err := oc.AsAdmin().Run("create").
						Args("-f", fmt.Sprintf("%s/crd-%s.yaml",
							SRIOVTestDataFixture, dev.ResourceName)).Execute()
					Expect(err).NotTo(HaveOccurred())
				}
				By("Creating SRIOV device plugin config map")
				err = oc.AsAdmin().Run("create").Args("-f",
					fmt.Sprintf("%s/%s", SRIOVTestDataFixture, sriovDPConfigMap),
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
							Args("-f", fmt.Sprintf("%s/crd-%s.yaml",
								SRIOVTestDataFixture, dev.ResourceName)).Execute()
						if err != nil {}
					}
					By("Deleting SRIOV device plugin daemonset")
					err := oc.AsAdmin().Run("delete").
						Args("-f", DevicePluginDaemonFixture, "-n", "kube-system").Execute()
					if err != nil {}

					By("Deleting SRIOV device plugin config map")
					err = oc.AsAdmin().Run("delete").Args("-f",
						fmt.Sprintf("%s/%s", SRIOVTestDataFixture, sriovDPConfigMap),
						"-n", "kube-system").Execute()

					By("Deleting SRIOV CNI daemonset")
					err = oc.AsAdmin().Run("delete").
						Args("-f", CNIDaemonFixture, "-n", "kube-system").Execute()

					By("Deleting SRIOV Admission Controller Server")
					err = oc.AsAdmin().Run("delete").
						Args("-f", AdmissionControllerServerDaemonFixture, "-n", "kube-system").Execute()

					By("Deleting SRIOV Admission Webhook")
					err = oc.AsAdmin().Run("delete").
						Args("-f", AdmissionControllerWebhookDaemonFixture, "-n", "kube-system").Execute()

					By("Deleting SRIOV Admission Controller ConfigMap")
					err = oc.AsAdmin().Run("delete").
						Args("-f", AdmissionControllerConfigMapFixture, "-n", "kube-system").Execute()

					By("Deleting SRIOV Admission Controller Service Account")
					err = oc.AsAdmin().Run("delete").
						Args("-f", AdmissionControllerSvcAcctDaemonFixture, "-n", "kube-system").Execute()

					By("Deleting SRIOV Admission Controller Service")
					err = oc.AsAdmin().Run("delete").
						Args("-f", AdmissionControllerSvcDaemonFixture, "-n", "kube-system").Execute()
				}
			}()

			time.Sleep(60 * time.Second)
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
						SRIOVTestDataFixture, n.ResourceName)).Execute()
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
					Args(fmt.Sprintf("testpod-%s", n.ResourceName),
						"--", "/bin/bash", "-c", "ip link show dev net1").Output()
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(ContainSubstring(fmt.Sprintf("does not exist")))
				Expect(out).To(ContainSubstring(fmt.Sprintf("mtu")))
				By(fmt.Sprintf("Pod net1 output: %s", out))

				err = wait.PollImmediate(e2e.Poll, 30*time.Second, func() (bool, error) {
					out, err = CheckPodAnnotations(oc, fmt.Sprintf( "testpod-%s", n.ResourceName))
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(ContainSubstring(fmt.Sprintf("does not exist")))
				Expect(out).To(ContainSubstring(fmt.Sprintf("labels")))
				Expect(out).To(ContainSubstring(fmt.Sprintf("annotations")))
				oc.AsAdmin().Run("delete").Args("-f", fmt.Sprintf("%s/pod-%s.yaml",
					SRIOVTestDataFixture, n.ResourceName)).Execute()
			}

		})
	})
})
