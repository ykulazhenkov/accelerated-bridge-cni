module github.com/k8snetworkplumbingwg/accelerated-bridge-cni

go 1.18

require (
	github.com/Mellanox/sriovnet v1.1.0
	github.com/containernetworking/cni v1.1.2
	github.com/containernetworking/plugins v1.1.0
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.3.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/rs/zerolog v1.20.0
	github.com/spf13/afero v1.6.0
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.1-0.20211101163509-b10eb8fe5cf6
)

require (
	github.com/coreos/go-iptables v0.6.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/safchain/ethtool v0.0.0-20210803160452-9aa261dae9b1 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect
	golang.org/x/text v0.3.6 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/apimachinery v0.22.8 // indirect
	k8s.io/klog/v2 v2.9.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)

replace github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.3.0 => github.com/ykulazhenkov/network-attachment-definition-client v1.3.1-0.20221017130552-86fdd7c7d49f
