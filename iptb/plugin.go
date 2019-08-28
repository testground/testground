package iptb

import (
	local "github.com/ipfs/iptb-plugins/local"
	"github.com/ipfs/iptb/testbed"
)

func init() {
	// register the localipfs plugin in iptb.
	_, err := testbed.RegisterPlugin(testbed.IptbPlugin{
		From:        "<builtin>",
		NewNode:     local.NewNode,
		GetAttrList: local.GetAttrList,
		GetAttrDesc: local.GetAttrDesc,
		PluginName:  local.PluginName,
		BuiltIn:     true,
	}, false)

	if err != nil {
		panic(err)
	}
}
