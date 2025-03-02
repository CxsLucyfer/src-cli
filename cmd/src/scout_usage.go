package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/docker/docker/client"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/src-cli/internal/scout/usage"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

func init() {
	cmdUsage := `'src scout usage' is a tool that tracks resource usage for Sourcegraph instances.
    Part of the EXPERIMENTAL "src scout" tool.
    
    Examples
        List pods and resource usage in a Kubernetes deployment:
        $ src scout usage

        List containers and resource usage in a Docker deployment:
        $ src scout usage --docker

        Check usage for specific pod
        $ src scout usage --pod <podname>

        Check usage for specific container (docker only)
        $ src scout usage --container <containername>

        Add namespace if using namespace in a Kubernetes cluster
        $ src scout usage --namespace <namespace>
        
        Watch usage in real time 
        $ src scout usage --spy
    `

	flagSet := flag.NewFlagSet("usage", flag.ExitOnError)
	usageFunc := func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'src scout %s':\n", flagSet.Name())
		flagSet.PrintDefaults()
		fmt.Println(cmdUsage)
	}

	var (
		kubeConfig *string
		namespace  = flagSet.String("namespace", "", "(optional) specify the kubernetes namespace to use")
		pod        = flagSet.String("pod", "", "(optional) specify a single pod")
		container  = flagSet.String("container", "", "(optional) specify a single container")
		docker     = flagSet.Bool("docker", false, "(optional) using docker deployment")
		spy        = flagSet.Bool("spy", false, "(optional) see resource usage in real time")
	)

	if home := homedir.HomeDir(); home != "" {
		kubeConfig = flagSet.String(
			"kubeconfig",
			filepath.Join(home, ".kube", "config"),
			"(optional) absolute path to the kubeconfig file",
		)
	} else {
		kubeConfig = flagSet.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	handler := func(args []string) error {
		if err := flagSet.Parse(args); err != nil {
			return err
		}

		config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
		if err != nil {
			return errors.Wrap(err, "failed to load .kube config: ")
		}

		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return errors.Wrap(err, "failed to initiate kubernetes client: ")
		}

		metricsClient, err := metricsv.NewForConfig(config)
		if err != nil {
			return errors.Wrap(err, "failed to initiate metrics client")
		}

		var options []usage.Option

		if *namespace != "" {
			options = append(options, usage.WithNamespace(*namespace))
		}
		if *spy {
			options = append(options, usage.WithSpy(true))
		}
		if *pod != "" {
			options = append(options, usage.WithPod(*pod))
		}
		if *container != "" || *docker {
			if *container != "" {
				options = append(options, usage.WithContainer(*container))
			}

			dockerClient, err := client.NewClientWithOpts(client.FromEnv)
			if err != nil {
				return errors.Wrap(err, "error creating docker client: ")
			}

			return usage.Docker(context.Background(), *dockerClient, options...)
		}

		return usage.K8s(
			context.Background(),
			clientSet,
			metricsClient,
			config,
			options...,
		)
	}

	scoutCommands = append(scoutCommands, &command{
		flagSet:   flagSet,
		handler:   handler,
		usageFunc: usageFunc,
	})

}
