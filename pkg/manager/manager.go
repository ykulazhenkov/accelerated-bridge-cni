package manager

import (
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/rs/zerolog/log"
	"github.com/vishvananda/netlink"
	nl "github.com/vishvananda/netlink/nl"

	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/utils"
)

// Manager provides interface invoke sriov nic related operations
type Manager interface {
	SetupVF(conf *types.PluginConf, podifName string, cid string, netns ns.NetNS) (string, error)
	ReleaseVF(conf *types.PluginConf, podifName string, cid string, netns ns.NetNS) error
	ResetVFConfig(conf *types.PluginConf) error
	ApplyVFConfig(conf *types.PluginConf) error
	AttachRepresentor(conf *types.PluginConf) error
	DetachRepresentor(conf *types.PluginConf) error
}

type manager struct {
	nLink utils.Netlink
	sriov utils.SriovnetProvider
}

// NewManager returns an instance of manager
func NewManager() Manager {
	return &manager{
		nLink: &utils.NetlinkWrapper{},
		sriov: &utils.SriovnetWrapper{},
	}
}

// SetupVF sets up a VF in Pod netns
func (m *manager) SetupVF(conf *types.PluginConf, podifName, cid string, netns ns.NetNS) (string, error) {
	linkName := conf.OrigVfState.HostIFName

	linkObj, err := m.nLink.LinkByName(linkName)
	if err != nil {
		return "", fmt.Errorf("error getting VF netdevice with name %s", linkName)
	}

	// tempName used as intermediary name to avoid name conflicts
	tempName := fmt.Sprintf("%s%d", "temp_", linkObj.Attrs().Index)

	// 1. Set link down
	if err = m.nLink.LinkSetDown(linkObj); err != nil {
		return "", fmt.Errorf("failed to down vf device %q: %v", linkName, err)
	}

	// 2. Set temp name
	if err = m.nLink.LinkSetName(linkObj, tempName); err != nil {
		return "", fmt.Errorf("error setting temp IF name %s for %s", tempName, linkName)
	}

	macAddress := linkObj.Attrs().HardwareAddr.String()
	// 3. Set MAC address
	if conf.MAC != "" {
		hwaddr, err1 := net.ParseMAC(conf.MAC)
		macAddress = conf.MAC
		if err1 != nil {
			return "", fmt.Errorf("failed to parse MAC address %s: %v", conf.MAC, err)
		}

		// Save the original effective MAC address before overriding it
		conf.OrigVfState.EffectiveMAC = linkObj.Attrs().HardwareAddr.String()

		if err = m.nLink.LinkSetHardwareAddr(linkObj, hwaddr); err != nil {
			return "", fmt.Errorf("failed to set netlink MAC address to %s: %v", hwaddr, err)
		}
	}

	// 4. Set MTU
	if conf.MTU != 0 {
		prevMTU := linkObj.Attrs().MTU
		if err = m.nLink.LinkSetMTU(linkObj, conf.MTU); err != nil {
			return "", fmt.Errorf("failed to set MTU on VF %s: %v", linkObj.Attrs().Name, err)
		}
		log.Info().Msgf("VF link %s MTU set to %d", linkObj.Attrs().Name, conf.MTU)
		conf.OrigVfState.MTU = prevMTU
	}

	// 5. Change netns
	if err := m.nLink.LinkSetNsFd(linkObj, int(netns.Fd())); err != nil {
		return "", fmt.Errorf("failed to move IF %s to netns: %q", tempName, err)
	}

	if err := netns.Do(func(_ ns.NetNS) error {
		// 6. Set Pod IF name
		if err := m.nLink.LinkSetName(linkObj, podifName); err != nil {
			return fmt.Errorf("error setting container interface name %s for %s", linkName, tempName)
		}

		// 7. Bring IF up in Pod netns
		if err := m.nLink.LinkSetUp(linkObj); err != nil {
			return fmt.Errorf("error bringing interface up in container ns: %q", err)
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf("error setting up interface in container namespace: %q", err)
	}
	conf.ContIFNames = podifName

	return macAddress, nil
}

// ReleaseVF reset a VF from Pod netns and return it to init netns
func (m *manager) ReleaseVF(conf *types.PluginConf, podifName, cid string, netns ns.NetNS) error {
	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	if len(conf.ContIFNames) < 1 && len(conf.ContIFNames) != len(conf.OrigVfState.HostIFName) {
		return fmt.Errorf("number of interface names mismatch ContIFNames: %d HostIFNames: %d",
			len(conf.ContIFNames), len(conf.OrigVfState.HostIFName))
	}

	return netns.Do(func(_ ns.NetNS) error {
		// get VF device
		linkObj, err := m.nLink.LinkByName(podifName)
		if err != nil {
			return fmt.Errorf("failed to get netlink device with name %s: %q", podifName, err)
		}

		// shutdown VF device
		if err = m.nLink.LinkSetDown(linkObj); err != nil {
			return fmt.Errorf("failed to set link %s down: %q", podifName, err)
		}

		// rename VF device
		err = m.nLink.LinkSetName(linkObj, conf.OrigVfState.HostIFName)
		if err != nil {
			return fmt.Errorf("failed to rename link %s to host name %s: %q",
				podifName, conf.OrigVfState.HostIFName, err)
		}

		// reset effective MAC address
		if conf.MAC != "" {
			var hwaddr net.HardwareAddr
			hwaddr, err = net.ParseMAC(conf.OrigVfState.EffectiveMAC)
			if err != nil {
				return fmt.Errorf("failed to parse original effective MAC address %s: %v",
					conf.OrigVfState.EffectiveMAC, err)
			}

			if err = m.nLink.LinkSetHardwareAddr(linkObj, hwaddr); err != nil {
				return fmt.Errorf("failed to restore original effective netlink MAC address %s: %v",
					hwaddr, err)
			}
		}

		// reset MTU
		if conf.MTU != 0 {
			if err = m.nLink.LinkSetMTU(linkObj, conf.OrigVfState.MTU); err != nil {
				return fmt.Errorf("failed to set MTU on VF %s: %v", linkObj.Attrs().Name, err)
			}
			log.Info().Msgf("VF link %s MTU set to %d", linkObj.Attrs().Name, conf.OrigVfState.MTU)
		}

		// move VF device to init netns
		if err = m.nLink.LinkSetNsFd(linkObj, int(initns.Fd())); err != nil {
			return fmt.Errorf("failed to move interface %s to init netns: %v",
				conf.OrigVfState.HostIFName, err)
		}

		return nil
	})
}

func getVfInfo(link netlink.Link, id int) *netlink.VfInfo {
	attrs := link.Attrs()
	for i := range attrs.Vfs {
		if attrs.Vfs[i].ID == id {
			return &attrs.Vfs[i]
		}
	}
	return nil
}

// ApplyVFConfig configure a VF with parameters given in PluginConf
func (m *manager) ApplyVFConfig(conf *types.PluginConf) error {
	pfLink, err := m.nLink.LinkByName(conf.PFName)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.PFName, err)
	}

	// Save current the VF state before modifying it
	vfState := getVfInfo(pfLink, conf.VFID)
	if vfState == nil {
		return fmt.Errorf("failed to find vf %d for PF %s", conf.VFID, conf.PFName)
	}

	conf.OrigVfState.AdminMAC = vfState.Mac.String() // Save administrative MAC for restoring it later

	// Set mac address
	if conf.MAC != "" {
		var hwaddr net.HardwareAddr
		hwaddr, err = net.ParseMAC(conf.MAC)
		if err != nil {
			return fmt.Errorf("failed to parse MAC address %s: %v", conf.MAC, err)
		}

		if err = m.nLink.LinkSetVfHardwareAddr(pfLink, conf.VFID, hwaddr); err != nil {
			return fmt.Errorf("failed to set MAC address to %s: %v", hwaddr, err)
		}
	}

	return nil
}

// ResetVFConfig reset a VF to its original state
func (m *manager) ResetVFConfig(conf *types.PluginConf) error {
	pfLink, err := m.nLink.LinkByName(conf.PFName)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.PFName, err)
	}

	// Restore the original administrative MAC address
	if conf.MAC != "" {
		var hwaddr net.HardwareAddr
		hwaddr, err = net.ParseMAC(conf.OrigVfState.AdminMAC)
		if err != nil {
			return fmt.Errorf("failed to parse original administrative MAC address %s: %v",
				conf.OrigVfState.AdminMAC, err)
		}
		if err = m.nLink.LinkSetVfHardwareAddr(pfLink, conf.VFID, hwaddr); err != nil {
			return fmt.Errorf("failed to restore original administrative MAC address %s: %v", hwaddr, err)
		}
	}

	return nil
}

func (m *manager) AttachRepresentor(conf *types.PluginConf) error {
	bridge, err := m.nLink.LinkByName(conf.ActualBridge)
	if err != nil {
		return fmt.Errorf("failed to get bridge link %s: %v", conf.ActualBridge, err)
	}

	conf.Representor, err = m.sriov.GetVfRepresentor(conf.PFName, conf.VFID)
	if err != nil {
		return fmt.Errorf("failed to get VF's %d representor on NIC %s: %v", conf.VFID, conf.PFName, err)
	}

	var rep netlink.Link
	if rep, err = m.nLink.LinkByName(conf.Representor); err != nil {
		return fmt.Errorf("failed to get representor link %s: %v", conf.Representor, err)
	}

	if conf.MTU != 0 {
		conf.OrigRepState.MTU = rep.Attrs().MTU
		if err = m.nLink.LinkSetMTU(rep, conf.MTU); err != nil {
			return fmt.Errorf("failed to set MTU on representor %s: %v", conf.Representor, err)
		}
		log.Info().Msgf("Setting MTU %d on rep %s to the bridge %s", conf.MTU, conf.Representor, conf.ActualBridge)
	}

	if err = m.nLink.LinkSetUp(rep); err != nil {
		return fmt.Errorf("failed to set representor %s up: %v", conf.Representor, err)
	}

	log.Info().Msgf("Attaching rep %s to the bridge %s", conf.Representor, conf.ActualBridge)

	if err = m.nLink.LinkSetMaster(rep, bridge); err != nil {
		return fmt.Errorf("failed to add representor %s to bridge: %v", conf.Representor, err)
	}

	defer func() {
		if err != nil {
			_ = m.nLink.LinkSetNoMaster(rep)
		}
	}()

	// if VF has any VLAN config we should remove default vlan on port
	// if VLAN 1 explicitly requested we should not remove it from the port
	if conf.Vlan > 1 || len(conf.Trunk) > 0 {
		if err = utils.BridgePVIDVlanDel(m.nLink, rep, 1); err != nil {
			return fmt.Errorf("failed to remove default VLAN(1) for representor %s: %v", conf.Representor, err)
		}
	}

	if len(conf.Trunk) > 0 {
		if err = utils.BridgeTrunkVlanAdd(m.nLink, rep, conf.Trunk); err != nil {
			return fmt.Errorf("failed to add trunk VLAN for representor %s: %v", conf.Representor, err)
		}
		log.Info().Msgf("Setting multiple VLANs for rep %s: %v", conf.Representor, conf.Trunk)
	}

	if conf.Vlan > 0 {
		if err = utils.BridgePVIDVlanAdd(m.nLink, rep, conf.Vlan); err != nil {
			return fmt.Errorf("failed to set VLAN for representor %s: %v", conf.Representor, err)
		}
		log.Info().Msgf("Setting PVID VLAN for rep %s: %d", conf.Representor, conf.Vlan)
	}

	// add vlan config to uplink if configured to do so
	if conf.SetUplinkVlan {
		if err = m.addUplinkVlans(conf); err != nil {
			return fmt.Errorf("failed to add vlan to parent uplink %v", err)
		}
	}

	return nil
}

func (m *manager) addUplinkVlans(conf *types.PluginConf) error {
	var uplink netlink.Link
	var err error

	uplink, err = m.getPFUplinkOrBond(conf.PFName)
	if err != nil {
		return fmt.Errorf("failed to lookup PF for VF-Representor - PFName:%s: VF:%s %v",
			conf.PFName, conf.Representor, err)
	}

	var vlans []int
	if len(conf.Trunk) > 0 {
		vlans = conf.Trunk
	}

	if conf.Vlan > 0 {
		vlans = append(vlans, conf.Vlan)
	}

	if err = utils.BridgeTrunkVlanAdd(m.nLink, uplink, vlans); err != nil {
		return fmt.Errorf("failed to add VLANs to parent uplink %s: %v - %v", uplink.Attrs().Name, vlans, err)
	}
	log.Info().Msgf("Setting VLANs for uplink %s: %v", uplink.Attrs().Name, vlans)

	return nil
}

func (m *manager) getPFUplinkOrBond(pfname string) (netlink.Link, error) {
	var uplink netlink.Link
	var err error
	uplink, err = m.nLink.LinkByName(pfname)

	if err != nil {
		return nil, fmt.Errorf("failed to lookup PF %s: %v", pfname, err)
	}

	if uplink.Attrs().Slave != nil && uplink.Attrs().Slave.SlaveType() == "bond" {
		var bondLink netlink.Link
		bondLink, err = m.nLink.LinkByIndex(uplink.Attrs().MasterIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup bond interface from slave link master index - Name:%s MasterIndex:%d %v",
				uplink.Attrs().Name, uplink.Attrs().MasterIndex, err)
		}

		_, isBond := bondLink.(*netlink.Bond)
		if !isBond {
			return nil, fmt.Errorf("master index for link is not a bond - PFName:%s Master Link:%s %v",
				uplink.Attrs().Name, bondLink.Attrs().Name, err)
		}
		uplink = bondLink
		log.Debug().Msgf("Using bond master as uplink: %s", uplink.Attrs().Name)
	}

	return uplink, nil
}

func (m *manager) DetachRepresentor(conf *types.PluginConf) error {
	rep, err := m.nLink.LinkByName(conf.Representor)
	if err != nil {
		return fmt.Errorf("failed to get representor %s link: %v", conf.Representor, err)
	}

	if err = m.nLink.LinkSetDown(rep); err != nil {
		return fmt.Errorf("failed to set representor %s down: %v", conf.Representor, err)
	}

	// Restore MTU
	if conf.MTU != 0 {
		if err = m.nLink.LinkSetMTU(rep, conf.OrigRepState.MTU); err != nil {
			return fmt.Errorf("failed to set MTU on rep %s: %v", conf.Representor, err)
		}
		log.Info().Msgf("Restoring MTU %d on rep %s", conf.OrigRepState.MTU, conf.Representor)
	}

	// remove vlan config from uplink if configured to do so
	if conf.SetUplinkVlan {
		if err = m.deleteUplinkVlans(rep, conf); err != nil {
			log.Warn().Msgf("Failed to delete trunk VLANs to parent uplink %v", err)
		}
	}

	log.Info().Msgf("Detaching rep %s from the bridge %s", conf.Representor, conf.ActualBridge)
	return m.nLink.LinkSetNoMaster(rep)
}

func (m *manager) deleteUplinkVlans(rep netlink.Link, conf *types.PluginConf) error {
	var uplink netlink.Link
	var err error

	uplink, err = m.getPFUplinkOrBond(conf.PFName)
	if err != nil {
		return fmt.Errorf("failed to lookup PF for VF-Representor - PFName:%s: VF:%s %v",
			conf.PFName, conf.Representor, err)
	}

	var vlans []int
	if len(conf.Trunk) > 0 {
		vlans = conf.Trunk
	}

	if conf.Vlan > 0 {
		vlans = append(vlans, conf.Vlan)
	}

	var bridgeLink netlink.Link
	bridgeLink, err = m.nLink.LinkByIndex(uplink.Attrs().MasterIndex)
	if err != nil {
		return fmt.Errorf("failed to lookup bridge index for interface:%s: %d %v",
			uplink.Attrs().Name, uplink.Attrs().MasterIndex, err)
	}

	var currentbrif []string
	currentbrif, err = utils.GetBridgeInterfaces(bridgeLink.Attrs().Name)
	if err != nil {
		return fmt.Errorf("failed to get bridge interfaces:%s: %v",
			bridgeLink.Attrs().Name, err)
	}

	var allbrif map[int32][]*nl.BridgeVlanInfo
	allbrif, _ = utils.BridgeVlanList(m.nLink)

	var delvlans []int

	for _, vlan := range vlans {
		found := false

	foundvlan:
		for _, brif := range currentbrif {
			brlink, iferr := m.nLink.LinkByName(brif)
			if iferr != nil {
				log.Warn().Msgf("could not lookup netlink info for %s: %v", brif, iferr)
				continue
			}

			// skip the interface already being removed or the uplink, it obviously has these vlans.
			// we are looking for any others
			if rep.Attrs().Index == brlink.Attrs().Index || uplink.Attrs().Index == brlink.Attrs().Index {
				continue
			}

			for _, bvlaninfo := range allbrif[int32(brlink.Attrs().Index)] {
				if bvlaninfo.Vid == uint16(vlan) {
					found = true
					break foundvlan
				}
			}
		}
		if !found {
			delvlans = append(delvlans, vlan)
		}
	}

	if err = utils.BridgeTrunkVlanDel(m.nLink, uplink, delvlans); err != nil {
		return fmt.Errorf("failed to delete VLANs to parent uplink %s: %v - %v", uplink.Attrs().Name, delvlans, err)
	}
	log.Info().Msgf("Deleting VLANs for uplink %s: %v", uplink.Attrs().Name, delvlans)

	return nil
}
