package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/oecp/open-local-storage-service/pkg"

	"github.com/julienschmidt/httprouter"
	volumesnapshot "github.com/kubernetes-csi/external-snapshotter/client/v3/clientset/versioned"
	volumesnapshotinformers "github.com/kubernetes-csi/external-snapshotter/client/v3/informers/externalversions"
	clientset "github.com/oecp/open-local-storage-service/pkg/generated/clientset/versioned"
	"github.com/oecp/open-local-storage-service/pkg/generated/clientset/versioned/scheme"
	informers "github.com/oecp/open-local-storage-service/pkg/generated/informers/externalversions"
	"github.com/oecp/open-local-storage-service/pkg/metrics"
	"github.com/oecp/open-local-storage-service/pkg/scheduler/algorithm"
	"github.com/oecp/open-local-storage-service/pkg/scheduler/algorithm/predicates"
	"github.com/oecp/open-local-storage-service/pkg/scheduler/algorithm/priorities"
	"github.com/oecp/open-local-storage-service/pkg/scheduler/statussyncer"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	log "k8s.io/klog"
)

func NewExtenderServer(kubeClient kubernetes.Interface,
	lss clientset.Interface,
	snapClient volumesnapshot.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	localStorageInformerFactory informers.SharedInformerFactory,
	volumesnapshotInformerFactory volumesnapshotinformers.SharedInformerFactory,
	port int32, weights *pkg.NodeAntiAffinityWeight) *ExtenderServer {
	corev1Informers := kubeInformerFactory.Core().V1()
	storagev1Informers := kubeInformerFactory.Storage().V1()
	localStorageInformers := localStorageInformerFactory.Storage().V1alpha1()
	snapshotInformers := volumesnapshotInformerFactory.Snapshot().V1beta1()

	Ctx := algorithm.NewSchedulingContext(corev1Informers, storagev1Informers, localStorageInformers, snapshotInformers, weights)

	informersSyncd := make([]clientgocache.InformerSynced, 0)

	// setup storage class informer
	scInformer := storagev1Informers.StorageClasses().Informer()
	informersSyncd = append(informersSyncd, scInformer.HasSynced)
	log.Info("started storage class informer...")

	// setup pv informer
	pvInformer := corev1Informers.PersistentVolumes().Informer()
	informersSyncd = append(informersSyncd, pvInformer.HasSynced)
	log.Info("started PV informer...")

	// setup pvc informer
	pvcInformer := corev1Informers.PersistentVolumeClaims().Informer()
	informersSyncd = append(informersSyncd, pvcInformer.HasSynced)
	log.Info("started PVC informer...")

	// setup node informer
	nodeInformer := corev1Informers.Nodes().Informer()
	informersSyncd = append(informersSyncd, nodeInformer.HasSynced)
	log.Info("started Node informer...")

	// setup pod informer
	podInformer := corev1Informers.Pods().Informer()
	informersSyncd = append(informersSyncd, podInformer.HasSynced)
	log.Info("started Pod informer...")

	// setup node local storage informer
	localInformer := localStorageInformers.NodeLocalStorages().Informer()
	informersSyncd = append(informersSyncd, localInformer.HasSynced)
	log.Info("started NodeLocalStorage informer...")

	snapInformer := snapshotInformers.VolumeSnapshots().Informer()
	informersSyncd = append(informersSyncd, snapInformer.HasSynced)

	snapContentInformer := snapshotInformers.VolumeSnapshotContents().Informer()
	informersSyncd = append(informersSyncd, snapContentInformer.HasSynced)

	snapStorageClassInformer := snapshotInformers.VolumeSnapshotClasses().Informer()
	informersSyncd = append(informersSyncd, snapStorageClassInformer.HasSynced)

	// setup syncer
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: pkg.SchedulerName})

	syncer := statussyncer.NewStatusSyncer(recorder, lss, Ctx)
	e := &ExtenderServer{
		kubeClient:         kubeClient,
		localStorageClient: lss,
		snapClient:         snapClient,
		port:               port,
		Ctx:                Ctx,
		informersSyncd:     informersSyncd,
		syncer:             syncer}
	localInformer.AddEventHandler(clientgocache.ResourceEventHandlerFuncs{
		AddFunc:    e.onNodeLocalStorageAdd,
		UpdateFunc: e.onNodeLocalStorageUpdate,
		DeleteFunc: nil,
	})
	pvInformer.AddEventHandler(clientgocache.ResourceEventHandlerFuncs{
		AddFunc:    e.onPVAdd,
		UpdateFunc: e.onPVUpdate,
		DeleteFunc: e.onPVDelete,
	})
	pvcInformer.AddEventHandler(clientgocache.ResourceEventHandlerFuncs{

		AddFunc:    e.onPvcAdd,
		UpdateFunc: e.onPvcUpdate,
		DeleteFunc: e.onPvcDelete,
	})
	podInformer.AddEventHandler(clientgocache.ResourceEventHandlerFuncs{
		AddFunc:    e.onPodAdd,
		UpdateFunc: e.onPodUpdate,
		DeleteFunc: e.onPodDelete,
	})

	return e
}

type ExtenderServer struct {
	Ctx                    *algorithm.SchedulingContext
	kubeClient             kubernetes.Interface
	localStorageClient     clientset.Interface
	snapClient             volumesnapshot.Interface
	port                   int32
	informersSyncd         []clientgocache.InformerSynced
	syncer                 *statussyncer.StatusSyncer
	currentWorkingRoutines int32
}

func (e *ExtenderServer) Start(stopCh <-chan struct{}) {
	e.InitRouter()
	e.WaitForCacheSync(stopCh)
	e.currentWorkingRoutines = 0
	log.Infof("maxConcurrentWorkingRoutines was set to %d", MaxConcurrentWorkingRoutines)
	log.Info("started open-local-storage-service scheduler extender")
	go e.TriggerPendingPodReschedule(stopCh)
	<-stopCh
	log.Info("Shutting down open-local-storage-service scheduler extender")
}

func (e *ExtenderServer) InitRouter() {
	// Init Prometheus
	prometheus.MustRegister([]prometheus.Collector{
		metrics.VolumeGroupTotal,
		metrics.MountPointTotal,
		metrics.DeviceTotal,
		metrics.VolumeGroupUsedByLSS,
		metrics.MountPointAvailable,
		metrics.DeviceAvailable,
		metrics.DeviceBind,
		metrics.MountPointBind,
		metrics.AllocatedNum,
		metrics.LocalPV,
	}...)

	// Setting up the extender http server
	router := httprouter.New()
	AddVersion(router)
	AddMetrics(router)
	AddGetNodeCache(router, e.Ctx)
	AddPredicate(router, *predicates.NewPredicate(e.Ctx))
	AddPrioritize(router, *priorities.NewPrioritize(e.Ctx))
	AddSchedulingApis(router, e.Ctx)

	go func() {
		if e.port > 0 {
			log.Infof("starting http server on port %d", e.port)
			if err := http.ListenAndServe(fmt.Sprintf(":%d", e.port), router); err != nil {
				log.Fatal(err)
			}
			log.Infof("started http server on port %d", e.port)

		} else {
			log.Infof("port is %d, not starting up http server", e.port)
		}
	}()
}

func (e *ExtenderServer) WaitForCacheSync(stopCh <-chan struct{}) {
	// Wait for the caches to be synced before starting workers
	log.Info("Waiting for informer caches to sync")
	if ok := clientgocache.WaitForCacheSync(stopCh, e.informersSyncd...); !ok {
		log.Fatal("failed to wait for all informer caches to be synced")
	}
	log.Info("all informer caches are synced")
}

func (e *ExtenderServer) TriggerPendingPodReschedule(stopCh <-chan struct{}) {
	ticker := time.NewTicker(pkg.TriggerPendingPodCycle)
	opt := metav1.ListOptions{FieldSelector: pkg.PendingWithoutScheduledFieldSelector}
	for range ticker.C {
		podList, err := e.kubeClient.CoreV1().Pods("").List(context.Background(), opt)
		if err != nil {
			log.Errorf("list pod with FieldSelector %+v :%+v", pkg.PendingWithoutScheduledFieldSelector, err)
			continue
		}
		if len(podList.Items) == 0 {
			continue
		}
		count := 0
		for _, pod := range podList.Items {
			if count >= 300 {
				break
			}
			if pod.ObjectMeta.DeletionTimestamp != nil {
				continue
			}
			pvcs, err := algorithm.GetAllPodPvcs(&pod, e.Ctx, true)
			if err != nil {
				log.Errorf("failed to get pod pvcs: %s", err.Error())
				continue
			}
			needTrigger := true
			if len(pvcs) > 0 {
				for _, pvc := range pvcs {
					if pvc.Status.Phase != corev1.ClaimPending {
						needTrigger = false
						break
					}
				}
			}
			if needTrigger && len(pvcs) > 0 {
				log.Infof("starting trigger pending pod %s/%s reschedule", pod.Namespace, pod.Name)
				now := strconv.FormatInt(metav1.Now().Unix(), 10)
				patchData := map[string]interface{}{"metadata": map[string]map[string]string{"labels": {pkg.LabelReschduleTimestamp: now}}}
				playLoadBytes, err := json.Marshal(patchData)
				if err != nil {
					log.Errorf("json marshal %+v:%+v", patchData, err)
					continue
				}
				_, err = e.kubeClient.CoreV1().Pods(pod.Namespace).Patch(context.Background(), pod.Name, types.MergePatchType, playLoadBytes, metav1.PatchOptions{})
				if err != nil {
					log.Errorf("patch %+v label for pod %+v: %+v", pkg.LabelReschduleTimestamp, pod.Name, err)
					continue
				}
				log.Infof("pathed label %+v=%+v to pod %s/%s", pkg.LabelReschduleTimestamp, now, pod.Namespace, pod.Name)
				count++
				time.Sleep(time.Second * 1)
			}
		}

	}
	<-stopCh
}