package fullnode

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StuckPodDetection struct {
	available      func(pods []*corev1.Pod, minReady time.Duration, now time.Time) []*corev1.Pod
	collector      StatusCollector
	computeRollout func(maxUnavail *intstr.IntOrString, desired, ready int) int
}

func NewStuckDetection(collector StatusCollector) DriftDetection {
	return DriftDetection{
		available:      kube.AvailablePods,
		collector:      collector,
		computeRollout: kube.ComputeRollout,
	}
}

// StuckPods returns pods that are stuck on a block height due to a cometbft issue that manifests on sentries using horcrux.
func (d StuckPodDetection) StuckPods(ctx context.Context, crd *cosmosv1.CosmosFullNode) []*corev1.Pod {

	pods := d.collector.Collect(ctx, client.ObjectKeyFromObject(crd)).Synced().Pods()

	fmt.Println(pods[0])

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	getPodLogsLastLine(clientset, pods[0])

	//MORE TODO HERE

	return []*corev1.Pod{}
}

func getPodLogsLastLine(clientset *kubernetes.Clientset, pod *corev1.Pod) {
	podLogOpts := corev1.PodLogOptions{}
	logRequest := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)

	logStream, err := logRequest.Stream(context.Background())
	if err != nil {
		fmt.Printf("Error getting logs for pod %s: %v\n", pod.Name, err)
		return
	}
	defer logStream.Close()

	logBytes, err := ioutil.ReadAll(logStream)
	if err != nil {
		fmt.Printf("Error reading logs for pod %s: %v\n", pod.Name, err)
		return
	}

	logLines := strings.Split(strings.TrimRight(string(logBytes), "\n"), "\n")
	if len(logLines) > 0 {
		fmt.Println("Last line of logs for pod", pod.Name+":", logLines[len(logLines)-1])
	} else {
		fmt.Println("No logs found for pod", pod.Name)
	}
}
