package pushtxman

import "github.com/0xPolygonHermez/zkevm-node/config/types"

// Config is configuration for L2 claim transaction manager
type Config struct {
	//Enabled whether to enable this module
	Enabled               bool           `mapstructure:"Enabled"`
	FrequencyToMonitorTxs types.Duration `mapstructure:"FrequencyToMonitorTxs"`
	FullChainStatusAPI    string         `mapstructure:"FullChainStatusAPI"`
	NodeRpcs              []NodeRpc      `mapstructure:"NodeRpcs"`
}

type NodeRpc struct {
	Name    string `mapstructure:"Name"`
	ChainID uint   `mapstructure:"ChainID"`
	Url     string `mapstructure:"Url"`
}
