package runner

import (
	"io/ioutil"

	"sigs.k8s.io/yaml"
)

// ClusterInfo represents the local cluster information.
type ClusterInfo struct {
	URIs     []string `json:"uris"`
	Endpoint string   `json:"endpoint"`
	PID      int      `json:"pid"`
	LogsDir  string   `json:"logsDir"`

	XChainPreFundedAddr string            `json:"xChainPreFundedAddr"`
	XChainAddrs         map[string]string `json:"xChainAddrs"`
	PChainPreFundedAddr string            `json:"pChainPreFundedAddr"`
	PChainAddrs         map[string]string `json:"pChainAddrs"`
	CChainPreFundedAddr string            `json:"cChainPreFundedAddr"`
	CChainAddrs         map[string]string `json:"cChainAddrs"`
}

const fsModeWrite = 0o600

func (ci ClusterInfo) Save(p string) error {
	ob, err := yaml.Marshal(ci)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, ob, fsModeWrite)
}

// LoadClusterInfo loads the cluster info YAML file
// to parse it into "ClusterInfo".
func LoadClusterInfo(p string) (ClusterInfo, error) {
	ob, err := ioutil.ReadFile(p)
	if err != nil {
		return ClusterInfo{}, err
	}
	info := new(ClusterInfo)
	if err = yaml.Unmarshal(ob, info); err != nil {
		return ClusterInfo{}, err
	}
	return *info, nil
}
