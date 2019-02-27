package networkattachments

import (
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
	})
})

