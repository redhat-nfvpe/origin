package networkattachments

import (
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"fmt"
        exutil "github.com/openshift/origin/test/extended/util"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/apimachinery/pkg/util/wait"
	corev1 "k8s.io/api/core/v1"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[Area:Networking] Multus-CNI", func() {
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

		f1 := e2e.NewDefaultFramework("multus-ds")

		It("should create multus pod on each node", func() {
			Expect(checkMultusDaemonStatus(f1)).To(Succeed())
		})

		privileged := true
		procMount := corev1.DefaultProcMount
		runAsUser := int64(0)
		hostPathType := corev1.HostPathType(string(corev1.HostPathDirectory))
		multusCIPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: oc.Namespace(),
			},
			Spec: corev1.PodSpec{
                                RestartPolicy:                 corev1.RestartPolicyNever,
                                Containers: []corev1.Container{
                                        {
						Name: "multus-ci-container",
						Image: "s1061123/multus-ci-scripts:latest",
						ImagePullPolicy: "Always",
						SecurityContext: &corev1.SecurityContext{
							Privileged: &privileged,
							ProcMount: &procMount,
							RunAsUser: &runAsUser,
						},
						Args: []string{"multus-basic-openshift"},
						Stdin: true,
						StdinOnce: true,
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "FallbackToLogsOnError",
						TTY: true,
						VolumeMounts: []corev1.VolumeMount{
							{
								MountPath: "/host",
								Name: "host",
							},
						},
					},
                                },
				HostNetwork: true,
				HostPID: true,
				Tolerations: []corev1.Toleration{
					{
						Effect: "NoExecute",
						Key: "node.kubernetes.io/not-ready",
						Operator: "Exists",
					}, {
						Effect: "NoExecute",
						Key: "node.kubernetes.io/unreachable",
						Operator: "Exists",
					},
				},
				Volumes: []corev1.Volume{
					{
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
								Type: &hostPathType,
							},
						},
						Name: "host",
					},
				},
                        },
		}

		It("should create multus cni related files on each node", func() {
			nodeList, _ := f1.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})

			nodeNames := make([]string, 0)
			lastNode := ""
			for _, node := range nodeList.Items {
				nodeNames = append(nodeNames, node.Name)
				lastNode = node.Name
			}

			for _, node := range nodeNames {
				pod := *multusCIPod
				pod.Spec.NodeName = node
				pod.ObjectMeta.Name = fmt.Sprintf("multus-ci-script-%s", node)
				created, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Create(&pod)
				e2e.Logf("run pod at %s: %v", lastNode, err) //XXX (need to revisit)
				Expect(err).NotTo(HaveOccurred())
				err = wait.PollImmediate(e2e.Poll, 3*time.Minute, func() (bool, error) {
					retrievedPod, err := oc.AdminKubeClient().CoreV1().
						Pods(oc.Namespace()).
						Get(created.Name, metav1.GetOptions{})
					if err != nil {
						return false, nil
					}
					return (retrievedPod.Status.Phase == corev1.PodSucceeded),nil
				})
				Expect(err).NotTo(HaveOccurred())
			}
			// in case error, show log?
			// waitForDeployerToComplete
		})
	})
})
