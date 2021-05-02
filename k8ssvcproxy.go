package main

import (
	"context"
	"flag"
	"fmt"
	k8shttp "k8ssvcproxy/http"
	"k8ssvcproxy/proxypass"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	apicorev1 "k8s.io/api/core/v1"
)

// CounterServiceMap is a map for storing Service to portforward object
type CounterServiceMap = struct {
	sync.RWMutex
	m map[string]proxypass.PortForwardOptions
}

// K8SProxyServiceConfig is an object to store all connfigurations
type K8SProxyServiceConfig struct {
	kubeconfig                  *string
	namespace                   *string
	listenAddress               *string
	litenPort                   *int
	services                    *apicorev1.ServiceList
	matchVersionKubeConfigFlags *cmdutil.MatchVersionFlags
	counterServiceMap           *CounterServiceMap
	ioStreams                   genericclioptions.IOStreams
}

// InitConfig is inializing configuration
func (config *K8SProxyServiceConfig) InitConfig() {
	if home := homedir.HomeDir(); home != "" {
		config.kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		config.kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	config.namespace = flag.String("namespace", "default", "kubernetes namespace")

	config.listenAddress = flag.String("listen", "127.0.0.1", "default address to listen")

	config.litenPort = flag.Int("port", 8080, "default port to listen")

	flag.Parse()
	fmt.Printf("Connecting to %s using %s\n", *config.namespace, *config.kubeconfig)
	restConfig, err := clientcmd.BuildConfigFromFlags("", *config.kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err.Error())
	}

	services, err := clientset.CoreV1().Services(*config.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	config.services = services

	if len(services.Items) == 0 {
		fmt.Println("No Services found!")
		os.Exit(1)
	}

	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.Namespace = config.namespace
	config.matchVersionKubeConfigFlags = cmdutil.NewMatchVersionFlags(kubeConfigFlags)

}

func main() {

	config := &K8SProxyServiceConfig{
		ioStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		counterServiceMap: &CounterServiceMap{
			sync.RWMutex{},
			make(map[string]proxypass.PortForwardOptions),
		},
	}

	config.InitConfig()

	httpHandler := &k8shttp.Proxy{
		RegisterdServices: &k8shttp.RegisteredServicesAndPorts{
			sync.RWMutex{},
			make(map[string]string),
		},
	}

	f := cmdutil.NewFactory(config.matchVersionKubeConfigFlags)

	for _, value := range config.services.Items {
		fmt.Printf("Service to proxy: %s\n", value.Name)
		if len(value.Spec.Ports) != 1 {
			fmt.Printf("Servce %s has more than 1 port: %d\n", value.Name, len(value.Spec.Ports))
		} else if value.Spec.Type != "ClusterIP" {
			fmt.Printf("Servce %s has wrong type: %s\n", value.Name, value.Spec.Type)
		} else {
			go runPortForward(value, f, config.ioStreams, config.listenAddress, config.counterServiceMap)
		}
	}
	go registerHttpListener(config.counterServiceMap, *config.listenAddress, *config.namespace, httpHandler.RegisterdServices)

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", *config.listenAddress, *config.litenPort), httpHandler)
	if err != nil {
		panic(err)
	}

}
func runPortForward(
	value apicorev1.Service, f cmdutil.Factory,
	ioStreams genericclioptions.IOStreams,
	listenAddress *string, m *CounterServiceMap) {

	pf := proxypass.NewCmdPortForward(f, ioStreams, *listenAddress)

	m.RLock()
	m.m[value.Name] = *pf
	m.RUnlock()

	fmt.Printf("Forwarding %s: %d\n", value.Name, value.Spec.Ports[0].Port)
	argv := []string{fmt.Sprintf("service/%s", value.Name), fmt.Sprintf(":%d", value.Spec.Ports[0].Port)}

	err := pf.Complete(f, argv)

	if err != nil {
		panic(err.Error())
	}

	err = pf.RunPortForward()

	if err != nil {
		panic(err.Error())
	}
}

func registerHttpListener(m *CounterServiceMap, listenAddress string, namespace string, registered *k8shttp.RegisteredServicesAndPorts) {

	listOfRegisteredServices := []string{}
	for {
		m.RLock()
		for k, v := range m.m {
			if contains(listOfRegisteredServices, k) {
				continue
			}

			fmt.Printf("Trying to register servce: %s\n", k)
			ports, err := v.PortForwarder.GetPorts()
			if err != nil {
				fmt.Printf("Connection for Service %s still in progress\n", k)
				continue
			}
			fmt.Printf("Servce %s registered on port %d\n", k, ports[0].Local)
			listOfRegisteredServices = append(listOfRegisteredServices, k)

			if err != nil {
				panic(err)
			}
			serviceName := fmt.Sprintf("%s.%s.svc.cluster.local:%d", k, namespace, ports[0].Remote)
			registered.RLock()
			registered.ServiceToPortMap[serviceName] = fmt.Sprintf("%s:%d", listenAddress, ports[0].Local)
			registered.RUnlock()
		}
		m.RUnlock()
		time.Sleep(1000 * time.Millisecond)
	}

}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
