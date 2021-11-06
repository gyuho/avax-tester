// Package avax impelemnts Avalanche constants and types.
package avax

import (
	"bytes"
	"text/template"
)

type Flag struct {
	NodeName           string
	NodeID             string
	Binary             string
	LogLevel           string
	NetworkID          string
	PublicIP           string
	HTTPPort           int
	SnowSampleSize     int
	SnowQuorumSize     int
	DBDir              string
	StakingEnabled     bool
	StakingPort        int
	BootstrapIPs       string
	BootstrapIDs       string
	StakingTLSKeyFile  string
	StakingTLSCertFile string
}

const tmplFlag = `# commands for {{.NodeName}}, {{.NodeID}}
kill -9 $(lsof -t -i:{{.HTTPPort}})
kill -9 $(lsof -t -i:{{.StakingPort}}){{if .StakingEnabled}}
openssl x509 -in {{.StakingTLSCertFile}} -text -noout{{end}}
cd ${HOME}/go/src/github.com/ava-labs/avalanchego
{{.Binary}} \
--log-level={{.LogLevel}} \
--network-id={{.NetworkID}} \
--public-ip={{.PublicIP}} \
--http-port={{.HTTPPort}} \
--snow-sample-size={{.SnowSampleSize}} \
--snow-quorum-size={{.SnowQuorumSize}} \
--db-dir={{.DBDir}} \
--staking-enabled={{.StakingEnabled}} \
--staking-port={{.StakingPort}} \
--bootstrap-ips={{.BootstrapIPs}} \
--bootstrap-ids={{.BootstrapIDs}}{{if .StakingEnabled}} \
--staking-tls-key-file={{.StakingTLSKeyFile}} \
--staking-tls-cert-file={{.StakingTLSCertFile}}{{end}}
`

func (f Flag) String() string {
	t := template.Must(template.New("tmplFlag").Parse(tmplFlag))
	var buf bytes.Buffer
	if err := t.Execute(&buf, f); err != nil {
		panic(err)
	}
	return buf.String()
}
