/*
Copyright 2018 The Kubernetes Authors.

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

package testsuites

import (
	"bufio"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"

	"github.com/outscale-dev/osc-bsu-csi-driver/tests/e2e/driver"
	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"
)

func getDf(data string) string {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	stop := 0
	for scanner.Scan() {
		text := scanner.Text()
		stop++
		if stop == 2 {
			singleSpacePattern := regexp.MustCompile(`\s+`)
			return singleSpacePattern.ReplaceAllString(text, " ")
		}
	}
	return ""
}

func getMetrics(data string, ns string, pvc string) string {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	// The target is to find and get data from following lines
	//kubelet_volume_stats_available_bytes{namespace="dynamic-p",persistentvolumeclaim="bsu-claim"} 4.12649472e+09
	//kubelet_volume_stats_capacity_bytes{namespace="dynamic-p",persistentvolumeclaim="bsu-claim"} 4.160421888e+09
	//kubelet_volume_stats_used_bytes{namespace="dynamic-p",persistentvolumeclaim="bsu-claim"} 1.7149952e+07
	//kubelet_volume_stats_inodes{namespace="dynamic-p",persistentvolumeclaim="bsu-claim"} 262144
	//kubelet_volume_stats_inodes_free{namespace="dynamic-p",persistentvolumeclaim="bsu-claim"} 262132
	//kubelet_volume_stats_inodes_used{namespace="dynamic-p",persistentvolumeclaim="bsu-claim"} 12
	KUBELET_VOLUME_PREFIX := "kubelet_volume_stats_"
	var kubelet_volume_stats_available_bytes,
		kubelet_volume_stats_capacity_bytes,
		kubelet_volume_stats_inodes,
		kubelet_volume_stats_inodes_free,
		kubelet_volume_stats_inodes_used,
		kubelet_volume_stats_used_bytes string
	stop := 0
	for scanner.Scan() {
		text := scanner.Text()
		if stop == 6 {
			return fmt.Sprintf("%s %s %s %s %s %s",
				kubelet_volume_stats_available_bytes,
				kubelet_volume_stats_capacity_bytes,
				kubelet_volume_stats_inodes,
				kubelet_volume_stats_inodes_free,
				kubelet_volume_stats_inodes_used,
				kubelet_volume_stats_used_bytes)
		}
		if strings.HasPrefix(text, KUBELET_VOLUME_PREFIX) &&
			strings.Contains(text, "namespace=\""+ns+"\"") &&
			strings.Contains(text, "persistentvolumeclaim=\""+pvc+"\"") {
			fields := strings.Split(text, "}")
			if len(fields) > 1 {
				value_str := strings.TrimSpace(strings.Split(text, "}")[1])
				flt, _, err := big.ParseFloat(value_str, 10, 0, big.ToNearestEven)
				if err != nil {
					panic(err)
				}

				value := new(big.Int)
				value, _ = flt.Int(value)

				fmt.Printf("value : %v ; %v\n", value, text)
				if strings.Contains(text, "kubelet_volume_stats_available_bytes") {
					kubelet_volume_stats_available_bytes = fmt.Sprintf("%d", value)
					stop++
				} else if strings.Contains(text, "kubelet_volume_stats_capacity_bytes") {
					kubelet_volume_stats_capacity_bytes = fmt.Sprintf("%d", value)
					stop++
				} else if strings.Contains(text, "kubelet_volume_stats_used_bytes") {
					kubelet_volume_stats_used_bytes = fmt.Sprintf("%d", value)
					stop++
				} else if strings.Contains(text, "kubelet_volume_stats_inodes_free") {
					kubelet_volume_stats_inodes_free = fmt.Sprintf("%d", value)
					stop++
				} else if strings.Contains(text, "kubelet_volume_stats_inodes_used") {
					kubelet_volume_stats_inodes_used = fmt.Sprintf("%d", value)
					stop++
				} else if strings.Contains(text, "kubelet_volume_stats_inodes") {
					kubelet_volume_stats_inodes = fmt.Sprintf("%d", value)
					stop++
				}
			}
		}
	}
	return ""
}

type DynamicallyProvisionedStatsPodTest struct {
	CSIDriver driver.DynamicPVTestDriver
	Pod       PodDetails
}

func (t *DynamicallyProvisionedStatsPodTest) Run(client clientset.Interface, namespace *v1.Namespace, f *framework.Framework) {
	customImage := "centos"
	tDeployment, cleanup := t.Pod.SetupDeployment(client, namespace, t.CSIDriver, customImage)
	// defer must be called here for resources not get removed before using them
	for i := range cleanup {
		defer cleanup[i]()
	}

	By("deploying the deployment")
	tDeployment.Create()

	By("checking that the pod is running")
	tDeployment.WaitForPodReady()

	pods, err := e2edeployment.GetPodsForDeployment(client, tDeployment.deployment)
	framework.ExpectNoError(err)

	pod_host_ip := pods.Items[0].Status.HostIP
	pvc_ns := tDeployment.namespace.Name
	pvc_name := tDeployment.deployment.Spec.Template.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName

	By("checking volume stats using /metrics ")
	err = nil
	stdout := ""
	stderr := ""
	metrics_kubelet_volume_stats := ""
	for i := 0; i < 20; i++ {
		fmt.Printf("Retries no %d", i)
		time.Sleep(10 * time.Second)
		cmd := []string{
			"curl",
			"-s",
			fmt.Sprintf("http://%s:10255/metrics", pod_host_ip),
		}
		stdout, stderr, err = f.ExecCommandInContainerWithFullOutput(tDeployment.podName, pods.Items[0].Spec.Containers[0].Name, cmd...)
		fmt.Printf("stdout %v, stderr %v, err %v\n", stdout, stderr, err)
		metrics_kubelet_volume_stats = getMetrics(stdout, pvc_ns, pvc_name)
		if metrics_kubelet_volume_stats != "" {
			break
		}
	}
	if err != nil {
		panic(err.Error())
	} else if metrics_kubelet_volume_stats == "" {
		panic("kubelet_volume_stats are empty")
	}

	By("checking volume stats using df ")
	//df --output=avail,size,itotal,iavail,iused,used --block-size=1 /mnt/test-1 | tail -1 | tr -s ' ' > /metrics.df
	df_stats := ""
	for i := 0; i < 10; i++ {
		fmt.Printf("Retries no %d", i)
		time.Sleep(10 * time.Second)
		cmd := []string{
			"df",
			"--output=avail,size,itotal,iavail,iused,used",
			"--block-size=1",
			"/mnt/test-1",
		}
		stdout, stderr, err = f.ExecCommandInContainerWithFullOutput(tDeployment.podName, pods.Items[0].Spec.Containers[0].Name, cmd...)
		fmt.Printf("stdout %v, stderr %v, err %v\n", stdout, stderr, err)
		df_stats = getDf(stdout)
		if df_stats != "" {
			break
		}
	}
	if err != nil {
		panic(err.Error())
	} else if df_stats == "" {
		panic("df_stats are empty")
	}

	fmt.Printf("df_stats %v\n", df_stats)
	fmt.Printf("metrics_kubelet_volume_stats  %v\n", metrics_kubelet_volume_stats)
	framework.ExpectEqual(df_stats, metrics_kubelet_volume_stats)

}
