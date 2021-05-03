package main

import (
	"context"
	"flag"
	"fmt"
	"k8ssvcproxy/fileupdater"
	k8shttp "k8ssvcproxy/http"
	"k8ssvcproxy/proxypass"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	m map[string]*proxypass.PortForwardOptions
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
	updateHosts                 *bool
}

// InitConfig is inializing configuration
func (config *K8SProxyServiceConfig) InitConfig() {
	if home := homedir.HomeDir(); home != "" {
		config.kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		config.kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	config.namespace = flag.String("namespace", "default", "comma-separated kubernetes namespace]")

	config.listenAddress = flag.String("listen", "127.0.0.1", "default address to listen")

	config.litenPort = flag.Int("port", 8080, "default port to listen")

	config.updateHosts = flag.Bool("update-hosts", false, "udate /etc/hosts")

	flag.Parse()
	log.Println("Connecting to ", *config.namespace, "using ", *config.kubeconfig)
	restConfig, err := clientcmd.BuildConfigFromFlags("", *config.kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err.Error())
	}

	ns := strings.Split(*config.namespace, ",")
	for _, v := range ns {
		services, err := clientset.CoreV1().Services(v).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		if config.services == nil {
			config.services = services
		} else {
			config.services.Items = append(config.services.Items, services.Items...)
		}
	}

	if len(config.services.Items) == 0 {
		log.Println("No Services found!")
		os.Exit(1)
	}

	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	config.matchVersionKubeConfigFlags = cmdutil.NewMatchVersionFlags(kubeConfigFlags)

}

func main() {

	config := &K8SProxyServiceConfig{
		ioStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		counterServiceMap: &CounterServiceMap{
			sync.RWMutex{},
			make(map[string]*proxypass.PortForwardOptions),
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
		log.Println("Service to proxy:", value.Name, value.Namespace)
		if value.Spec.Type != "ClusterIP" {
			log.Println("Servce", value.Name, value.Namespace, "has wrong type:", value.Spec.Type)
		} else {
			runPortForward(value, f, config.ioStreams, config.listenAddress, config.counterServiceMap)
		}
	}

	for k, v := range config.counterServiceMap.m {
		log.Println("Running: ", k)
		go run(k, v)
	}

	registerHttpListener(config.counterServiceMap, *config.listenAddress, *config.namespace, httpHandler.RegisterdServices)

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", *config.listenAddress, *config.litenPort), httpHandler)
	if err != nil {
		panic(err)
	}

}

func run(service string, pf *proxypass.PortForwardOptions) {
	if pf == nil {
		log.Println("Service is nil:", service)
		return
	}
	err := pf.RunPortForward()
	if err != nil {
		log.Println("Error with service", service, err.Error())
	}
}

func runPortForward(
	value apicorev1.Service, f cmdutil.Factory,
	ioStreams genericclioptions.IOStreams,
	listenAddress *string, servcesMap *CounterServiceMap) {

	if len(value.Spec.Ports) != 1 {
		log.Println("Servce", value.Name, "has more than 1 port:", len(value.Spec.Ports), "Using first:", value.Spec.Ports[0].Port)
	}

	pf := proxypass.NewCmdPortForward(value.Namespace, ioStreams, *listenAddress)
	log.Println("Forwarding", value.Name, value.Namespace, "->", value.Spec.Ports[0].Port)
	argv := []string{fmt.Sprintf("service/%s", value.Name), fmt.Sprintf(":%d", value.Spec.Ports[0].Port)}

	err := pf.Complete(f, argv)

	if err != nil {
		panic(err.Error())
	}
	k := value.Name + "." + value.Namespace
	servcesMap.m[k] = pf
}

func registerHttpListener(servicesMap *CounterServiceMap, listenAddress string, namespace string, registered *k8shttp.RegisteredServicesAndPorts) {

	listOfRegisteredServices := []string{}
	for {
		for k, v := range servicesMap.m {
			if contains(listOfRegisteredServices, k) {
				continue
			}

			if v.PortForwarder.GetOuterPortForward() == nil {
				log.Println("Connection for Service", k, "is still in progress")
				continue
			}
			ports, err := v.PortForwarder.GetPorts()
			if err != nil {
				log.Println("Connection for Service", k, "is still in progress")
				continue
			}
			fmt.Printf("Servce %s registered on port %d\n", k, ports[0].Local)
			listOfRegisteredServices = append(listOfRegisteredServices, k)

			if err != nil {
				panic(err)
			}
			serviceName := fmt.Sprintf("%s.svc.cluster.local:%d", k, ports[0].Remote)
			registered.ServiceToPortMap[serviceName] = fmt.Sprintf("%s:%d", listenAddress, ports[0].Local)
		}
		if len(listOfRegisteredServices) == len(servicesMap.m) {
			log.Println("All services was registered!")
			fileupdater.PrintEtcHosts(registered)
			return
		}
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
