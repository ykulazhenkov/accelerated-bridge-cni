package plugin

import (
	"github.com/containernetworking/cni/pkg/types"
	specTypes "github.com/containernetworking/cni/pkg/types/040"
	latestTypes "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ipam"
)

// IPAM represents limited subset of functions from ipam package
type IPAM interface {
	ExecAdd(plugin string, netconf []byte) (types.Result, error)
	ExecDel(plugin string, netconf []byte) error
	ConfigureIface(ifName string, res *specTypes.Result) error
}

// ipamWrapper wrapper for ipam package
type ipamWrapper struct{}

// ExecAdd is a wrapper for ipam.ExecAdd
func (i *ipamWrapper) ExecAdd(plugin string, netconf []byte) (types.Result, error) {
	return ipam.ExecAdd(plugin, netconf)
}

// ExecDel is a wrapper for ipam.ExecDel
func (i *ipamWrapper) ExecDel(plugin string, netconf []byte) error {
	return ipam.ExecDel(plugin, netconf)
}

// ConfigureIface is a wrapper for ipam.ConfigureIface
func (i *ipamWrapper) ConfigureIface(ifName string, res *specTypes.Result) error {
	r, err := res.GetAsVersion(latestTypes.ImplementedSpecVersion)
	if err != nil {
		return err
	}
	return ipam.ConfigureIface(ifName, r.(*latestTypes.Result))
}
