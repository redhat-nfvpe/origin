package networkattachments

import (
	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[Area:Networking] Network-attachment-definition", func() {
	defer GinkgoRecover()

	InNetworkAttachmentContext(func() {
		It("network-attachment-definition-pass", func() {
			Expect(10).To(Equal(10))
		})
/*
		It("network-attachment-definition-fail", func() {
			Expect(10).To(Equal(1))
		})
*/

		f1 := e2e.NewDefaultFramework("multus-ds")

		It("should create Multus pod on each node", func() {
			Expect(checkMultusDaemonStatus(f1)).To(Succeed())
		})
	})
})
