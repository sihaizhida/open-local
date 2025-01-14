/*
Copyright © 2021 Alibaba Group Holding Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"time"

	"github.com/alibaba/open-local/pkg/controller"
	clientset "github.com/alibaba/open-local/pkg/generated/clientset/versioned"
	informers "github.com/alibaba/open-local/pkg/generated/informers/externalversions"
	"github.com/alibaba/open-local/pkg/signals"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	opt = controllerOption{}
)

var Cmd = &cobra.Command{
	Use:   "controller",
	Short: "command for starting a controller",
	Run: func(cmd *cobra.Command, args []string) {
		err := Start(&opt)
		if err != nil {
			log.Fatalf("error :%s, quitting now\n", err.Error())
		}
	},
}

func init() {
	opt.addFlags(Cmd.Flags())
}

// Start will start controller
func Start(opt *controllerOption) error {
	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(opt.Master, opt.Kubeconfig)
	if err != nil {
		return fmt.Errorf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("Error building kubernetes clientset: %s", err.Error())
	}

	localClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	localInformerFactory := informers.NewSharedInformerFactory(localClient, time.Second*30)

	controller := controller.NewController(kubeClient, localClient, kubeInformerFactory.Core().V1().Nodes(), localInformerFactory.Csi().V1alpha1().NodeLocalStorages(), localInformerFactory.Csi().V1alpha1().NodeLocalStorageInitConfigs(), opt.InitConfig)

	kubeInformerFactory.Start(stopCh)
	localInformerFactory.Start(stopCh)

	log.Info("Starting open-local controller")
	if err = controller.Run(2, stopCh); err != nil {
		return fmt.Errorf("Error running controller: %s", err.Error())
	}
	log.Info("Quitting now")
	return nil
}
