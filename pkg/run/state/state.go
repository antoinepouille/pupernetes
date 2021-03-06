package state

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

// State keeps track of the current stats
type State struct {
	sync.RWMutex

	apiServerProbeLastError string
	dnsLastError            string
	kubectlApplied          bool
	ready                   bool

	kubeletProbeFailures  int
	kubeletAPIPodRunning  int
	kubeletLogsPodRunning int

	promVersion prometheus.Gauge

	promStateReady            prometheus.Gauge
	promKubeletAPIPodRunning  prometheus.Gauge
	promKubeletLogsPodRunning prometheus.Gauge
	promKubeletProbeFailures  prometheus.Counter
	promReadyDNSFailures      prometheus.Counter
}

// NewState instantiate a state with the associated prometheus metrics
func NewState() (*State, error) {
	s := &State{
		promVersion: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "pupernetes_version",
			Help:        "Pupernetes version",
			ConstLabels: prometheus.Labels{},
			// TODO record all versions in labels. hyperkube: "1.10.1", etcd: "3.11.1", ...
		}),
		promStateReady: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pupernetes_ready",
			Help: "Boolean for pupernetes readiness",
		}),
		promKubeletAPIPodRunning: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pupernetes_kubelet_api_pods_running",
			Help: "Number of kubelet API pods running",
		}),
		promKubeletLogsPodRunning: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pupernetes_kubelet_logs_pods_running",
			Help: "Number of kubelet logs pods running",
		}),
		promKubeletProbeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pupernetes_kubelet_probe_failures",
			Help: "Total number of kubelet probe failures",
		}),
		promReadyDNSFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pupernetes_dns_failures",
			Help: "Total number of dns query failures",
		}),
	}
	err := registerCollectors(s.promVersion, s.promStateReady, s.promKubeletAPIPodRunning, s.promKubeletLogsPodRunning, s.promKubeletProbeFailures, s.promReadyDNSFailures)
	if err != nil {
		return nil, err
	}
	s.promVersion.Inc()
	return s, nil
}

func registerCollectors(collectors ...prometheus.Collector) error {
	for _, c := range collectors {
		err := prometheus.Register(c)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsReady returns if the kube-apiserver is available and the manifests are applied
func (s *State) IsReady() bool {
	s.RLock()
	defer s.RUnlock()
	return s.ready
}

// SetReady is the trigger to mark pupernetes as ready.
// It notify systemd of its readiness
func (s *State) SetReady() {
	s.Lock()
	s.ready = true
	s.Unlock()
	// Ignore errors
	notifySystemd()
	s.promStateReady.Set(1)
}

// SetKubectlApplied mark the state when kubectl apply successfully returned
func (s *State) SetKubectlApplied() {
	s.Lock()
	s.kubectlApplied = true
	s.Unlock()
}

// IsKubectlApplied returns true when kubectl is already applied
func (s *State) IsKubectlApplied() bool {
	s.RLock()
	defer s.RUnlock()
	return s.kubectlApplied
}

// SetAPIServerProbeLastError keep track of the latest error message and display only
// if there is a a diff from the last record
func (s *State) SetAPIServerProbeLastError(msg string) {
	s.Lock()
	if s.apiServerProbeLastError != msg {
		glog.Infof("Kubenertes apiserver not ready yet: %s", msg)
		s.apiServerProbeLastError = msg
	}
	s.Unlock()
}

// SetDNSLastError keep track of the latest error message and display only
// if there is a a diff from the last record
func (s *State) SetDNSLastError(msg string) {
	s.Lock()
	if s.dnsLastError != msg {
		glog.Infof("Kubenertes dns not ready yet: %s", msg)
		s.dnsLastError = msg
	}
	s.Unlock()
	s.promReadyDNSFailures.Inc()
}

// IncKubeletProbeFailures increment the number of kubelet failures
func (s *State) IncKubeletProbeFailures() {
	s.Lock()
	s.kubeletProbeFailures++
	s.Unlock()
	s.promKubeletProbeFailures.Inc()
}

// GetKubeletProbeFail returns the number of kubelet failures
func (s *State) GetKubeletProbeFail() int {
	s.RLock()
	defer s.RUnlock()
	return s.kubeletProbeFailures
}

// SetKubeletAPIPodRunning keep track of the number of kubelet Pods and display only
// if there is a a diff from the last record
func (s *State) SetKubeletAPIPodRunning(nb int) {
	s.Lock()
	if s.kubeletAPIPodRunning != nb {
		glog.Infof("Kubelet API reports %d running pods", nb)
		s.kubeletAPIPodRunning = nb
	}
	s.Unlock()
	s.promKubeletAPIPodRunning.Set(float64(nb))
}

// SetKubeletLogsPodRunning keep track of the number of kubelet Pods in /var/log/pods and display only
// if there is a a diff from the last record
func (s *State) SetKubeletLogsPodRunning(nb int) {
	s.Lock()
	if s.kubeletLogsPodRunning != nb {
		glog.Infof("Kubelet log reports %d running pods", nb)
		s.kubeletLogsPodRunning = nb
	}
	s.Unlock()
	s.promKubeletLogsPodRunning.Set(float64(nb))
}

// GetKubeletLogsPodRunning returns the number of kubelet Pods in /var/log/pods
func (s *State) GetKubeletLogsPodRunning() int {
	s.RLock()
	defer s.RUnlock()
	return s.kubeletLogsPodRunning
}
