package priorities

import (
	"fmt"
	"time"

	"github.com/oecp/open-local-storage-service/pkg/scheduler/algorithm"
	"github.com/oecp/open-local-storage-service/pkg/scheduler/algorithm/cache"
	corev1 "k8s.io/api/core/v1"
	log "k8s.io/klog"
	utiltrace "k8s.io/utils/trace"
)

// CountMatch picks the node whose amount of device/mount point best fulfill the amount of pvc requests
func CountMatch(ctx *algorithm.SchedulingContext, pod *corev1.Pod, node *corev1.Node) (int, error) {
	trace := utiltrace.New(fmt.Sprintf("Scheduling[CountMatch] %s/%s", pod.Namespace, pod.Name))
	defer trace.LogIfLong(50 * time.Millisecond)
	containReadonlySnapshot := false
	err, _, mpPVCs, devicePVCs := algorithm.GetPodPvcs(pod, ctx, true, containReadonlySnapshot)
	if err != nil {
		return MinScore, err
	}
	nc := ctx.ClusterNodeCache.GetNodeCache(node.Name)
	if nc == nil {
		return 0, fmt.Errorf("failed to get node cache by name %s", node.Name)
	}
	var scoreMP, scoreDevice int
	freeMPCount, err := freeMountPoints(nc)
	if len(mpPVCs) > 0 && freeMPCount > 0 {
		if err != nil {
			return 0, err
		}
		scoreMP = int(float64(len(mpPVCs)) * float64(MaxScore) / float64(freeMPCount))
		log.V(5).Infof("[CountMatch]node %s got %d out of %d", node.Name, scoreMP, MaxScore)
	}
	freeDeviceCount, err := freeDevices(nc)
	if len(devicePVCs) > 0 && freeDeviceCount > 0 {
		if err != nil {
			return 0, err
		}
		scoreDevice = int(float64(len(devicePVCs)) * float64(MaxScore) / float64(freeDeviceCount))
		log.V(5).Infof("[CountMatch]node %s got %d out of %d", node.Name, scoreDevice, MaxScore)
	}
	return (scoreMP + scoreDevice) / 2.0, nil
}

func freeMountPoints(nc *cache.NodeCache) (int, error) {

	freeMPs := make([]cache.ExclusiveResource, 0)
	for _, mp := range nc.MountPoints {
		if !mp.IsAllocated {
			freeMPs = append(freeMPs, mp)
		}
	}
	return len(freeMPs), nil
}

func freeDevices(nc *cache.NodeCache) (int, error) {
	freeDevices := make([]cache.ExclusiveResource, 0)
	for _, device := range nc.Devices {
		if !device.IsAllocated {
			freeDevices = append(freeDevices, device)
		}
	}
	return len(freeDevices), nil

}
