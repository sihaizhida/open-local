package predicates

import (
	"fmt"
	"time"

	"github.com/oecp/open-local-storage-service/pkg/scheduler/algorithm"
	"github.com/oecp/open-local-storage-service/pkg/scheduler/algorithm/algo"
	"github.com/oecp/open-local-storage-service/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	log "k8s.io/klog"
	utiltrace "k8s.io/utils/trace"
)

// CapacityPredicate checks if local storage on a node matches the persistent volume claims, follow rules are applied:
// 1. pvc contains vg or mount point or device claim
// 2. node free size must larger or equal to pvcs
// 3. for pvc of type mount point/device:
//	 a. must contains more mount points than pvc count
func CapacityPredicate(ctx *algorithm.SchedulingContext, pod *corev1.Pod, node *corev1.Node) (bool, error) {
	trace := utiltrace.New(fmt.Sprintf("Scheduling[CapacityPredicate] %s/%s", pod.Namespace, pod.Name))
	defer trace.LogIfLong(50 * time.Millisecond)

	containReadonlySnapshot := false
	err, lvmPVCs, mpPVCs, devicePVCs := algorithm.GetPodPvcs(pod, ctx, true, containReadonlySnapshot)

	if err != nil {
		return false, err
	}

	var fits bool
	if len(lvmPVCs) > 0 {
		trace.Step("Computing AllocateLVMVolume")

		fits, _, err = algo.AllocateLVMVolume(pod, lvmPVCs, node, ctx)
		if err != nil {
			log.Error(err)
			return false, err
		} else if fits == false {
			return false, nil
		}
	}

	if len(mpPVCs) > 0 {
		trace.Step("Computing AllocateMountPointVolume")

		fits, _, err = algo.AllocateMountPointVolume(pod, mpPVCs, node, ctx)
		if err != nil {
			log.Error(err)
			return false, err
		} else if fits == false {
			return false, nil
		}
	}

	if len(devicePVCs) > 0 {
		trace.Step("Computing AllocateDeviceVolume")

		fits, _, err = algo.AllocateDeviceVolume(pod, devicePVCs, node, ctx)
		if err != nil {
			log.Error(err)
			return false, err
		} else if fits == false {
			return false, nil
		}
	}

	containReadonlySnapshot = true
	err, lvmPVCs, _, _ = algorithm.GetPodPvcs(pod, ctx, true, containReadonlySnapshot)
	if err != nil {
		return false, err
	}
	// if pod has snapshot pvc
	// select all snapshot pvcs, and check if nodes of them are the same
	if utils.ContainsSnapshotPVC(lvmPVCs) == true {
		var fits bool
		var err error
		if fits, err = algo.ProcessSnapshotPVC(lvmPVCs, node, ctx); err != nil {
			return false, err
		}
		if fits == false {
			return false, nil
		}
	}

	if len(lvmPVCs) <= 0 && len(mpPVCs) <= 0 && len(devicePVCs) <= 0 {
		log.V(4).Infof("no open-local-storage-service volume request on pod %s, skipped", pod.Name)
		return true, nil
	}

	return true, nil
}