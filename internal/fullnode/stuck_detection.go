package fullnode

import (
	"context"
	"fmt"
	"io"
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

func NewStuckDetection(collector StatusCollector) StuckPodDetection {
	return StuckPodDetection{
		available:      kube.AvailablePods,
		collector:      collector,
		computeRollout: kube.ComputeRollout,
	}
}

// StuckPods returns pods that are stuck on a block height due to a cometbft issue that manifests on sentries using horcrux.
func (d StuckPodDetection) StuckPods(ctx context.Context, crd *cosmosv1.CosmosFullNode) []*corev1.Pod {
	pods := d.collector.Collect(ctx, client.ObjectKeyFromObject(crd)).Synced().Pods()

	for i, pod := range pods {
		config, err := rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		receivedString := getPodLogsLastLine(clientset, pod)
		fmt.Println(receivedString)
		podIsStuck := isPodStuck(receivedString)

		//MORE TODO HERE
		if podIsStuck {
			pods = removeElement(pods, i)
		}
	}
	return pods
}

func isPodStuck(receivedString string) bool {
	if strings.Contains(receivedString, "SignerListener: Connected") {
		timeInLog, err := extractTimeFromLog(receivedString)
		if err != nil {
			fmt.Println("Error parsing time from log:", err)
			return true
		}

		currentTime := time.Now().UTC()

		logTimeToday := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(),
			timeInLog.Hour(), timeInLog.Minute(), timeInLog.Second(), timeInLog.Nanosecond(), currentTime.Location())

		timeDiff := currentTime.Sub(logTimeToday)

		if timeDiff >= time.Minute {
			return true
		}
	}

	return false
}

func extractTimeFromLog(log string) (time.Time, error) {
	parts := strings.Fields(log)

	const timeLayout = "3:04PM"
	parsedTime, err := time.Parse(timeLayout, parts[0])
	if err != nil {
		return time.Time{}, err
	}

	return parsedTime, nil
}

func getPodLogsLastLine(clientset *kubernetes.Clientset, pod *corev1.Pod) string {
	podLogOpts := corev1.PodLogOptions{}
	logRequest := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)

	logStream, err := logRequest.Stream(context.Background())
	if err != nil {
		fmt.Printf("Error getting logs for pod %s: %v\n", pod.Name, err)
		return ""
	}
	defer logStream.Close()

	logBytes, err := io.ReadAll(logStream)
	if err != nil {
		fmt.Printf("Error reading logs for pod %s: %v\n", pod.Name, err)
		return ""
	}

	logLines := strings.Split(strings.TrimRight(string(logBytes), "\n"), "\n")
	if len(logLines) > 0 {
		return logLines[len(logLines)-1]
	}
	return ""
}

func removeElement(slice []*corev1.Pod, index int) []*corev1.Pod {
	return append(slice[:index], slice[index+1:]...)
}
