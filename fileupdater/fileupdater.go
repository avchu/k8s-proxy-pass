package fileupdater

import (
	"html/template"
	k8shttp "k8ssvcproxy/http"
	"log"
	"os"
	"runtime"
	"strings"
)

const hostsTemplate = `
### START K8SSVCPROXY ###
{{range $key, $value := .HostsMap}}{{$value}}    {{$key}}
{{end}}### END K8SSVCPROXY ###
`

type Hosts struct {
	HostsMap map[string]string
}

// UpdateEtcHosts is a function to update /etc/hosts for linux or System32/etc/hosts for Windows
func UpdateEtcHosts(services *k8shttp.RegisteredServicesAndPorts) {
	etcHostsFile := "/etc/hosts"
	if runtime.GOOS == "Windows" {
		log.Println("Windows is not supporting now")
	}
	f, err := os.OpenFile(etcHostsFile,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.WriteString("text to append\n"); err != nil {
		log.Println(err)
	}

}

// PrintEtcHosts is a function to print hosts map
func PrintEtcHosts(services *k8shttp.RegisteredServicesAndPorts) {
	hostsMap := &Hosts{
		make(map[string]string),
	}

	for k, v := range services.ServiceToPortMap {
		hostsMap.HostsMap[strings.Split(k, ":")[0]] = strings.Split(v, ":")[0]
	}

	tmpl, err := template.New("etc-hosts").Parse(hostsTemplate)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(os.Stdout, hostsMap)
	if err != nil {
		panic(err)
	}
}
