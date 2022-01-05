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

	XChainSecondaryAddresses map[string]string `json:"xChainSecondaryAddresses"`
	PChainSecondaryAddresses map[string]string `json:"pChainSecondaryAddresses"`

	EwoqWallet Wallet   `json:"ewoqWallet"`
	Wallets    []Wallet `json:"wallets"`
}

type Wallet struct {
	Name            string `json:"name"`
	PrivateKey      string `json:"privateKey"`
	PrivateKeyBytes []byte `json:"privateKeyBytes"`

	CommonAddress string `json:"commonAddress"`
	ShortAddress  string `json:"shortAddress"`
	XChainAddress string `json:"xChainAddress"`
	XChainBalance uint64 `json:"xChainBalance"`
	PChainAddress string `json:"pChainAddress"`
	PChainBalance uint64 `json:"pChainBalance"`
	CChainAddress string `json:"cChainAddress"`
	CChainBalance uint64 `json:"cChainBalance"`
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
