package sriovnetwork

import (
	"time"

        exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[Area:Networking] Cluster Network Operator", func() {
	defer GinkgoRecover()

	InNetworkAttachmentContext(func() {
		oc := exutil.NewCLI("sriov", exutil.KubeConfigPath())


		It("should be able to create SRIOV device plugin and SRIOV CNI daemonsets", func() {
			f1 := oc.KubeFramework()

			By("Patching Network.operator with SRIOV config")
			err := oc.AsAdmin().Run("patch").Args("Network.operator.openshift.io", "cluster", "-p", `{"spec": {"additionalNetworks": [{"type": "Raw", "name": "sriov-network", "namespace": "default", "rawCNIConfig": "{type: sriov, name: sriov-net}"}]}}`, "--type", "merge").Execute()
			if err != nil {
				e2e.Failf("Unable to initialize SRIOV daemonsets")
			}

			By("Waiting for SRIOV daemonsets become ready")
			err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
				ds, err := f1.ClientSet.AppsV1().DaemonSets("openshift-sriov").
					Get("sriov-device-plugin", metav1.GetOptions{})
				if err != nil {
					return false, nil
				}

				desired := ds.Status.DesiredNumberScheduled
				scheduled := ds.Status.CurrentNumberScheduled
				ready := ds.Status.NumberReady
				return (desired == scheduled && desired == ready), nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
