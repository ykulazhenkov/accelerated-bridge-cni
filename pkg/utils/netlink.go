package utils

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	nl "github.com/vishvananda/netlink/nl"
)

const (
	linkTypeBridge = "bridge"
)

// Netlink represents limited subset of functions from netlink package
type Netlink interface {
	LinkByName(string) (netlink.Link, error)
	LinkByIndex(index int) (netlink.Link, error)
	LinkSetVfHardwareAddr(netlink.Link, int, net.HardwareAddr) error
	LinkSetHardwareAddr(netlink.Link, net.HardwareAddr) error
	LinkSetUp(netlink.Link) error
	LinkSetDown(netlink.Link) error
	LinkSetNsFd(netlink.Link, int) error
	LinkSetName(netlink.Link, string) error
	LinkSetMaster(netlink.Link, netlink.Link) error
	LinkSetNoMaster(netlink.Link) error
	BridgeVlanAdd(netlink.Link, uint16, bool, bool, bool, bool) error
	BridgeVlanDel(netlink.Link, uint16, bool, bool, bool, bool) error
	LinkSetMTU(netlink.Link, int) error
	BridgeVlanList() (map[int32][]*nl.BridgeVlanInfo, error)
}

// NetlinkWrapper wrapper for netlink package
type NetlinkWrapper struct {
}

// LinkByName is a wrapper for netlink.LinkByName
func (n *NetlinkWrapper) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

// LinkByIndex is a wrapper for netlink.LinkByIndex
func (n *NetlinkWrapper) LinkByIndex(index int) (netlink.Link, error) {
	return netlink.LinkByIndex(index)
}

// LinkSetVfHardwareAddr is a wrapper for netlink.LinkSetVfHardwareAddr
func (n *NetlinkWrapper) LinkSetVfHardwareAddr(link netlink.Link, vf int, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetVfHardwareAddr(link, vf, hwaddr)
}

// LinkSetHardwareAddr is a wrapper for netlink.LinkSetHardwareAddr
func (n *NetlinkWrapper) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetHardwareAddr(link, hwaddr)
}

// LinkSetMTU is a wrapper for netlink.LinkSetMTU
func (n *NetlinkWrapper) LinkSetMTU(link netlink.Link, mtu int) error {
	return netlink.LinkSetMTU(link, mtu)
}

// LinkSetUp is a wrapper for netlink.LinkSetUp
func (n *NetlinkWrapper) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

// LinkSetDown is a wrapper for netlink.LinkSetDown
func (n *NetlinkWrapper) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

// LinkSetNsFd is a wrapper for netlink.LinkSetNsFd
func (n *NetlinkWrapper) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

// LinkSetName is a wrapper for netlink.LinkSetName
func (n *NetlinkWrapper) LinkSetName(link netlink.Link, name string) error {
	return netlink.LinkSetName(link, name)
}

// LinkSetMaster is a wrapper for netlink.LinkSetMaster
func (n *NetlinkWrapper) LinkSetMaster(link, master netlink.Link) error {
	return netlink.LinkSetMaster(link, master)
}

// LinkSetNoMaster is a wrapper for netlink.LinkSetNoMaster
func (n *NetlinkWrapper) LinkSetNoMaster(link netlink.Link) error {
	return netlink.LinkSetNoMaster(link)
}

// BridgeVlanAdd is a wrapper for netlink.BridgeVlanAdd
func (n *NetlinkWrapper) BridgeVlanAdd(link netlink.Link, vid uint16, pvid, untagged, self, master bool) error {
	return netlink.BridgeVlanAdd(link, vid, pvid, untagged, self, master)
}

// BridgeVlanDel is a wrapper for netlink.BridgeVlanDel
func (n *NetlinkWrapper) BridgeVlanDel(link netlink.Link, vid uint16, pvid, untagged, self, master bool) error {
	return netlink.BridgeVlanDel(link, vid, pvid, untagged, self, master)
}

func (n *NetlinkWrapper) BridgeVlanList() (map[int32][]*nl.BridgeVlanInfo, error) {
	return netlink.BridgeVlanList()
}

// BridgePVIDVlanAdd configure port VLAN id for link
func BridgePVIDVlanAdd(nlink Netlink, link netlink.Link, vlanID int) error {
	// pvid, egress untagged
	return nlink.BridgeVlanAdd(link, uint16(vlanID), true, true, false, true)
}

// BridgePVIDVlanDel remove port VLAN id for link
func BridgePVIDVlanDel(nlink Netlink, link netlink.Link, vlanID int) error {
	// pvid, egress untagged
	return nlink.BridgeVlanDel(link, uint16(vlanID), true, true, false, true)
}

// BridgeTrunkVlanAdd configure vlan trunk on link
func BridgeTrunkVlanAdd(nlink Netlink, link netlink.Link, vlans []int) error {
	// egress tagged
	for _, vlanID := range vlans {
		if err := nlink.BridgeVlanAdd(link, uint16(vlanID), false, false, false, true); err != nil {
			return err
		}
	}
	return nil
}

// BridgeTrunkVlanDel remove vlans from trunk on link
func BridgeTrunkVlanDel(nlink Netlink, link netlink.Link, vlans []int) error {
	// egress tagged
	for _, vlanID := range vlans {
		if err := nlink.BridgeVlanDel(link, uint16(vlanID), false, false, false, true); err != nil {
			return err
		}
	}
	return nil
}

func BridgeVlanList(nlink Netlink) (map[int32][]*nl.BridgeVlanInfo, error) {
	return nlink.BridgeVlanList()
}

// GetParentBridgeForLink returns linux bridge if provided link belongs to any.
// if provided link has a parent interface (e.g. interface is a part of a bond) will return a bridge
// to which parent interface belongs to
func GetParentBridgeForLink(nLink Netlink, link netlink.Link) (netlink.Link, error) {
	master := link
	var err error
	for master.Type() != linkTypeBridge {
		master, err = getMasterInterface(nLink, master)
		if err != nil {
			return nil, err
		}
	}
	return master, nil
}

// getMasterInterface returns a master interface for the link if it exists
func getMasterInterface(nLink Netlink, link netlink.Link) (netlink.Link, error) {
	if link.Attrs().MasterIndex == 0 {
		return nil, fmt.Errorf("link %s has no master", link.Attrs().Name)
	}
	return nLink.LinkByIndex(link.Attrs().MasterIndex)
}
