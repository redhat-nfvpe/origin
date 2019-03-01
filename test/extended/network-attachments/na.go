package networkattachments

import (
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"fmt"
        exutil "github.com/openshift/origin/test/extended/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[Area:Networking] Network-attachment-definition", func() {
	defer GinkgoRecover()

	InNetworkAttachmentContext(func() {
		oc := exutil.NewCLI("network-attachment", exutil.KubeConfigPath())
/*
		f := oc.AsAdmin().KubeFramework()
		anotherNsName := f.BaseName + "-b"
		anotherNs, err := f.CreateNamespace(anotherNsName, map[string]string{
			"ns-name": "anotherNsName",
		})
		Expect(err).NotTo(HaveOccurred())
		f.AddNamespacesToDelete(anotherNs)

*/
		It("network-attachment-definition-text", func() {
			configPath := exutil.FixturePath("testdata", "network-attachments", "net-attach-def-text.yaml")
                        By(fmt.Sprintf("creating pod w/network-attachment from a file %q", configPath))
			// Todo: we may split to call create net-attach-def/pod (due to privilege)
                        err := oc.AsAdmin().Run("create").Args("-f", configPath).Execute()
			Expect(err).NotTo(HaveOccurred())

		})

		It("network-attachment-definition-text-ifname", func() {
			configPath := exutil.FixturePath("testdata", "network-attachments", "net-attach-def-text-ifname.yaml")
                        By(fmt.Sprintf("creating pod w/network-attachment from a file %q", configPath))
			// Todo: we may split to call create net-attach-def/pod (due to privilege)
                        err := oc.AsAdmin().Run("create").Args("-f", configPath).Execute()
			Expect(err).NotTo(HaveOccurred())

		})

/*
		It("network-attachment-definition-text-ns", func() {
			configPath := exutil.FixturePath("testdata", "network-attachments", "net-attach-def-text-ns.yaml")
                        By(fmt.Sprintf("creating pod w/network-attachment from a file %q", configPath))
			// Todo: we may split to call create net-attach-def/pod (due to privilege)
                        err := oc.AsAdmin().Run("create").Args("-f", configPath).Execute()
			Expect(err).NotTo(HaveOccurred())

		})
*/

		It("network-attachment-definition-json", func() {
			configPath := exutil.FixturePath("testdata", "network-attachments", "net-attach-def-json.yaml")
                        By(fmt.Sprintf("creating pod w/network-attachment from a file %q", configPath))
			// Todo: we may split to call create net-attach-def/pod (due to privilege)
                        err := oc.AsAdmin().Run("create").Args("-f", configPath).Execute()
			Expect(err).NotTo(HaveOccurred())

		})

		It("network-attachment-definition-json-ifname", func() {
			configPath := exutil.FixturePath("testdata", "network-attachments", "net-attach-def-json-ifname.yaml")
                        By(fmt.Sprintf("creating pod w/network-attachment from a file %q", configPath))
			// Todo: we may split to call create net-attach-def/pod (due to privilege)
                        err := oc.AsAdmin().Run("create").Args("-f", configPath).Execute()
			Expect(err).NotTo(HaveOccurred())

		})

/*
		It("network-attachment-definition-json-ns", func() {
			configPath := exutil.FixturePath("testdata", "network-attachments", "net-attach-def-json-ns.yaml")
                        By(fmt.Sprintf("creating pod w/network-attachment from a file %q", configPath))
			// Todo: we may split to call create net-attach-def/pod (due to privilege)
                        err := oc.AsAdmin().Run("create").Args("-f", configPath).Execute()
			Expect(err).NotTo(HaveOccurred())

		})
*/

		f1 := e2e.NewDefaultFramework("multus-ds")

		It("should create Multus pod on each node", func() {
			Expect(checkMultusDaemonStatus(f1)).To(Succeed())
		})
	})
})
