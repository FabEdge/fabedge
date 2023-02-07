package agent

import (
	"io/ioutil"
	"os"
	"path"
	"text/template"
	"time"

	kubeproxy "k8s.io/kubernetes/cmd/kube-proxy/app"
)

const (
	kubeProxyConfigTemplate = `
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
bindAddress: 0.0.0.0
bindAddressHardFail: false
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: {{ .KubeConfig }}
  qps: 0
clusterCIDR: {{ .ClusterCIDR }}
configSyncPeriod: 0s
conntrack:
  maxPerCore: null
  min: null
  tcpCloseWaitTimeout: null
  tcpEstablishedTimeout: null
detectLocalMode: ""
enableProfiling: false
healthzBindAddress: ""
hostnameOverride: ""
iptables:
  masqueradeAll: false
  masqueradeBit: null
  minSyncPeriod: 0s
  syncPeriod: 0s
ipvs:
  excludeCIDRs: null
  minSyncPeriod: 0s
  scheduler: ""
  strictARP: false
  syncPeriod: 0s
  tcpFinTimeout: 0s
  tcpTimeout: 0s
  udpTimeout: 0s
metricsBindAddress: ""
mode: "{{ .Mode }}"
nodePortAddresses: null
oomScoreAdj: null
portRange: ""
showHiddenMetricsForVersion: ""
udpIdleTimeout: 0s
winkernel:
  enableDSR: false
  networkName: ""
  sourceVip: ""# 
`

	kubeconfigContent = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://127.0.0.1:10550
  name: default
contexts:
- context:
    cluster: default
    namespace: default
    user: default
  name: default
current-context: default
users:
- name: default
  user:
`
)

func (m *Manager) runKubeProxy() {
	configFilePath, err := m.writeKubeProxyConfigFiles()
	if err != nil {
		m.log.Error(err, "failed to write kube-proxy files")
		return
	}

	opts := kubeproxy.NewOptions()
	opts.ConfigFile = configFilePath
	if err := opts.Complete(); err != nil {
		m.log.Error(err, "failed to complete kube-proxy options")
		return
	}

	if err := opts.Validate(); err != nil {
		m.log.Error(err, "kube-proxy options are not valid")
		return
	}

	for {
		if err := opts.Run(); err != nil {
			m.log.Error(err, "kube-proxy failed to start")
		}
		time.Sleep(5 * time.Second)
	}
}

func (m *Manager) writeKubeProxyConfigFiles() (string, error) {
	kubeconfigPath := path.Join(m.Workdir, "kubeconfig")
	err := ioutil.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0644)
	if err != nil {
		return "", err
	}

	tpl, err := template.New("kube-proxy").Parse(kubeProxyConfigTemplate)
	if err != nil {
		return "", err
	}

	configFilePath := path.Join(m.Workdir, "kube-proxy.yaml")
	file, err := os.OpenFile(configFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data := struct {
		Mode        string
		ClusterCIDR string
		KubeConfig  string
	}{
		Mode:        m.Proxy.Mode,
		ClusterCIDR: m.Proxy.ClusterCIDR,
		KubeConfig:  kubeconfigPath,
	}
	err = tpl.Execute(file, data)

	return configFilePath, err
}
