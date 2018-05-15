package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type timeSlice []v1.Job

func (p timeSlice) Len() int {
	return len(p)
}

func (p timeSlice) Less(i, j int) bool {
	return p[i].Status.StartTime.Before(p[j].Status.StartTime)
}

func (p timeSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func main() {
	namespace := flag.String("namespace", "default", "Kubernetes namespace")
	inCluster := flag.Bool("in-cluster", true, "In-cluster deployment")
	labelSelector := flag.String("label", "", "Label selector to match")
	maxCount := flag.Int("max-count", 10, "Number of Jobs to remain")
	dryRun := flag.Bool("dry-run", true, "Only do dry run")

	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()

	fmt.Printf("In-cluster: %t\n", *inCluster)
	fmt.Printf("Namespace: %s\n", *namespace)
	fmt.Printf("Label selector: %s\n", *labelSelector)
	fmt.Printf("Dry run: %t\n", *dryRun)
	fmt.Printf("Max count: %d\n", *maxCount)
	fmt.Println()

	var config *rest.Config

	if *inCluster {
		c, err := rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		config = c
	} else {
		c, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
		config = c
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	batchClient := clientset.BatchV1().Jobs(*namespace)

	jobs, err := batchClient.List(metav1.ListOptions{LabelSelector: *labelSelector})
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("There are %d jobs with matching label in the cluster.\n\n", len(jobs.Items))

	sortedJobs := make(timeSlice, 0, len(jobs.Items))

	for _, job := range jobs.Items {
		name := job.GetName()
		succeeded := job.Status.Succeeded
		startTime := job.Status.StartTime

		fmt.Printf("Job Name: %s, Succeeded: %d, Start time: %s\n", name, succeeded, startTime)
		sortedJobs = append(sortedJobs, job)
	}

	sort.Sort(sortedJobs)

	fmt.Println()

	nJobsToRemove := max(len(sortedJobs)-*maxCount, 0)
	sortedJobs = sortedJobs[:nJobsToRemove]

	for _, job := range sortedJobs {
		if job.Status.Succeeded >= 1 {
			fmt.Printf("Deleting job %s.\n", job.Name)

			if *dryRun {
				fmt.Println("Dry run is enabled. Not deleting job.")
			} else {
				deletePolicy := metav1.DeletePropagationForeground
				if err := batchClient.Delete(job.Name, &metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				}); err != nil {
					panic(err)
				}
			}
		}
	}

	fmt.Println("Done.")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
