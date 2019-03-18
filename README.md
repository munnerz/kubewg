# kubewg

kubewg is a Kubernetes controller that allows you to configure and manage
[Wireguard] VPN configuration using a Kubernetes API server.

It introduces the following [CustomResourceDefinition] resources:

* **Network**: Represents a Wireguard VPN network.
* **Peer**: Represents a single Peer in a a Network. Each peer will be
  allocated an address in the network's subnet.
* **RouteBinding**: Represents additional route configuration that should be
  used by all members of the VPN network.

## How it works

kubewg is comprised of two core components:

* The control plane - which stores and generates wireguard configuration in the
  form of Peer resources

* `guardlet` - this component runs on each peer in your VPN. It watches the
  Kubernetes API for configuration stored on a Peer resource and applies it
  to the local Wireguard installation.

## Quick start

**Prerequisites**:

* 2 Linux or Darwin (OS X) systems, to test out the VPN
* Kubernetes 1.10+
* At least one peer must have an accessible network address for the Wireguard
  peers to connect to. If multiple have accessible network addresses, a mesh
  will be automatically formed.

### Installing wireguard on each peer

Currently, kubewg does not bundle an installation of wireguard.

Therefore you must install Wireguard according to your system/platform
installation method of choice.

You can find more information on install Wireguard on the [Wireguard website].

### Generating private keys and public keys

Currently, kubewg does not handle generation of private or public keys.

This guide will set up two VPN peers, `laptop` and `server`.

On each of these two VPN peers, you must generate a private key and the
corresponding public key to be used in later parts of the instructions.

After installing Wireguard, you can generate a private & public key using:

```shell
$ mkdir -p /etc/wireguard
$ wg genkey | tee /etc/wireguard/privatekey | wg pubkey > /etc/wireguard/publickey
```

This will create two files, named `privatekey` and `publickey` respectively.

You will need to repeat this step on each VPN peer you intend to add to the
network.

### Deploying the control plane

To get started, first install the kubewg controller into your Kubernetes
cluster:

```bash
$ kustomize build ./config | kubectl apply -f
```

This will install the Network, Peer and RouteBinding CustomResourceDefinitions
as well as the `kubewg-manager` controller into the `kubewg-system` namespace.

### Creating a Network

To configure your first VPN network, you must create a Network resource.
A network defines the CIDR that all VPN peers in the network should be assigned
addresses from.

This example will configure two VPN peers, `laptop` and `server`, both with a
static IP address.

```yaml
apiVersion: wg.mnrz.xyz/v1alpha1
kind: Network
metadata:
  name: examplenet
spec:
  subnet: 10.20.40.0/24
  allocations:
  - address: 10.20.40.10
    selector:
      names:
      - server
  - address: 10.20.40.20
    selector:
      names:
      - laptop
```

This defines a simple network with 2 configured peers, using the 10.20.40.0/24
subnet.

You must specify IP allocations for each Peer in the network using the
`allocations` stanza.

### Creating a Peer resource

Now that we have configured a Network, we must create Peer resources for each
peer that will be a member of the VPN network.

A Peer's `spec` stanza contains configuration detailing how to connect to the
particular peer, including the peer's public key and connection endpoint.

We first configure a `laptop` peer:

```yaml
apiVersion: wg.mnrz.xyz/v1alpha1
kind: Peer
metadata:
  name: laptop
spec:
  # We omit the 'host' portion of the listen address for the wireguard listener
  # to signal that this peer does not accept incoming connections from other
  # peers.
  # This is typically done when the peer sits behind a NAT firewall, e.g.
  # laptops and phones that may be portable.
  address: :12345
  # Enter the public key of the 'laptop' wireguard peer here.
  # This public key is generated in the 2nd step of the guide.
  # You should be able to find this file at /etc/wireguard/publickey.
  publicKey: <publickey-from-laptop>
```

The `server` peer will need to specify an accessible network address for the
`spec.address` field for the `laptop` peer to connect to:

```yaml
apiVersion: wg.mnrz.xyz/v1alpha1
kind: Peer
metadata:
  name: server
spec:
  address: wg.example.com:12345
  # Enter the public key of the 'server' wireguard peer here.
  # This public key is generated in the 2nd step of the guide.
  # You should be able to find this file at /etc/wireguard/publickey.
  publicKey: <publickey-from-server>
```

You **must** ensure that `wg.example.com:12345` is accessible from the `laptop`
peer for the laptop peer to successfully connect to the Wireguard server.

### Verifying route configuration has been generated

Once all the resources have been created, we can verify that peers have been
configured correctly by checking the `status` stanza of the resources:

```shell
$ kubectl describe network examplenet
...
Status:
  Allocations:
    Address:  10.20.40.10
    Name:     server
    Address:  10.20.40.20
    Name:     laptop
...

$ kubectl describe peer laptop
Status:
  Address:  10.20.40.20
  Network:  examplenet
  Peers:
    Allowed I Ps:
      10.20.40.10/32
    Endpoint:    wg.example.com:12345
    Name:        server
    Public Key:  <publickey-from-server>
...

$ kubectl describe peer server
...
Status:
  Address:  10.20.40.10
  Network:  examplenet
  Peers:
    Allowed I Ps:
      10.20.40.20/32
    Name:        laptop
    Public Key:  <publickey-from-laptop>
...
```

If you cannot see output similar to the above, check the logs from the
`kubewg-manager` component for indications of what may be failing.

### Configuring the guardlet on each VPN peer

Now that we have verified our Wireguard configuration for each peer is being
generated correctly, we must run the `guardlet` component on each VPN peer.

The `guardlet` is run using the `--peer-name` flag, and will automatically
apply the Wireguard configuration on the named Peer resource to the local
Wireguard installation, in order to configure the VPN network.

First, fetch a copy of `guardlet`, replacing OS with either `darwin` or `linux`
and ARCH with `amd64` or `mips64` appropriately:

```shell
$ curl -LO https://github.com/munnerz/kubewg/releases/download/v0.1.1/guardlet-OS_ARCH
$ chmod +x guardlet-OS_ARCH
$ sudo mv guardlet-OS_ARCH /usr/local/bin/guardlet
```

You can check for a full list of available releases on the [release pages].

There are a number of command line options that can be passed to the guardlet
to configure its behaviour:

```shell
$ guardlet --help
Usage of guardlet:
  -device-name string
    	Wireguard network interface name (default "utun9")
  -kubeconfig string
    	Paths to a kubeconfig. Only required if out-of-cluster.
  -master string
    	The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.
  -metrics-addr string
    	The address the metric endpoint binds to. (default ":8080")
  -os string
    	Host OS, used to determine commands to use to configure wireguard. Currently only darwin is supported. (default "darwin")
  -peer-name string
    	The name of this wireguard peer
  -private-key-file string
    	Path to a file containing the wireguard private key for this peer (default "/Users/James/go/src/github.com/munnerz/kubewg/privatekey")
  -sync-period duration
    	Adjust how often interface configuration is periodically resynced (default 5s)
  -use-kernel-module
    	If true, the 'ip' command will be used to create a wireguard interface using the Linux kernel driver
  -wg-binary string
    	Path to the wireguard 'wg' binary (default "wg")
```

There are a few special flags you must set when running the guardlet on
different platforms. You can see these detailed below.

You must configure the guardlet with a `kubeconfig` file that gives it
permissions to `get`, `list` and `watch` Peer resources.

The easiest way to do this is to create a ServiceAccount resource and grant
it permission with RBAC roles, and then retrieve the service account token from
the generated Secret resource:

```shell
$ kubectl create serviceaccount <peer-name>
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: wireguard-peer
rules:
  - apiGroups:
      - wg.mnrz.xyz
    resources:
      - peers
    verbs:
      - get
      - list
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: <peer-name>
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: wireguard-peer
subjects:
  - kind: ServiceAccount
    name: <peer-name>
    # update this if you created the service account in a different namespace
    namespace: default
```

You can then retrieve the service account token using `kubectl get secret`:

```shell
$ kubectl get secret -o yaml <peer-name>-token-<random text>
```

The token will be displayed, base64 encoded, as the `data.token` key.

You can use this secret to generate a `kubeconfig` file that can be passed to
guardlet.

#### Flags for OSX

```shell
$ sudo guardlet \
    --device-name utun9 \
    --kubeconfig path/to/kubeconfig/file \
    --os darwin \
    --peer-name <peer-name> \
```

When running on OSX, the [wireguard-go] userspace Wireguard implementation is
used.

#### Flags for Linux

```shell
$ sudo guardlet \
    --device-name wg0 \
    --kubeconfig path/to/kubeconfig/file \
    --os linux \
    --peer-name <peer-name> \
    --use-kernel-module true
```

When running on Linux, the native kernel module can be used to configure the
Wireguard interface for improved performance.

We must also set the `--device-name` flag to `wg[0-9]`, as the default of
`utun9` is only appropriate for `wireguard-go` systems.

### Verifying the VPN network is up

Once the guardlet has successfully reconciled its Peer configuration, you
should see the following message:

```
{"level":"info","ts":1552945374.877612,"logger":"controller","msg":"Reconciled wireguard configuration"}
```

This indicates that the configuration has been successfully applied to the
Wireguard peer.

From the `laptop` peer, you should now be able to ping the IP address of the
`server` Wireguard peer:

```shell
# Run this command from the 'laptop' Wireguard peer
$ ping 10.20.40.10
```

## Adding additional static routes

The kubewg `RouteBinding` resource can be used to configure static routes
within a VPN network.

This can be used to expose local networks that are routable via VPN peers,
or to create bridge points where traffic can be switched for the VPNs local
subnet.

An example of a RouteBinding that exposes the network `192.168.1.0/24` that is
local to the `server` VPN peer:

```yaml
apiVersion: wg.mnrz.xyz/v1alpha1
kind: RouteBinding
metadata:
  name: server-localnet-route
spec:
  routes:
  - 192.168.1.0/24
  network: examplenet
  selector:
    names:
    - server
```

This configuration will be automatically propagated to all VPN peers in the
examplenet network.

### Configuring a static route for the VPN subnet

In cases where you have multiple remote clients connecting to a central
Wireguard server that need to be able to communicate with each other, it can be
useful to set up a static route for the VPN network's subnet via the central
Wireguard server:

```yaml
apiVersion: wg.mnrz.xyz/v1alpha1
kind: RouteBinding
metadata:
  name: examplenet-subnet-default-route
spec:
  routes:
  - 10.20.40.0/24
  network: examplenet
  selector:
    names:
    - server
```

If a peer has a more direct route to another peer in the Wireguard mesh, it
will automatically take the shortest path to that peer.

Otherwise, this route will be used, allowing the central server to attempt to
route packets to the destination host.

## Generating a Wireguard .conf file

Some platforms may not support wireguard-go, such as tables and smartphones.

In order to provide support for managing configuration for these devices using
kubewg, an addtional `mkconf` command is provided.

This command should be passed a `--peer-name` and `--private-key` argument
which will be used to generate a .conf file that can be used by tools such as
`wg-quick`.

```shell
# Run this command from within a checked out copy of the kubewg repository
$ go run ./cmd/mkconf \
    --peer-name <peer-name> \
    --peer-namespace <peer-namespace> \
    --private-key "$(cat privatekey)"
```

The generated config file will be outputted to stdout.

### Generating a Wireguard config QR code

The `qrencode` utility can be used to generate a QR code containing the
generated .conf file.

This is especially useful when generating configuration for a mobile device
with a camera.

You can pipe the output of `mkconf` into `qrencode` in order to print a QR code
to your terminal display:

```shell
# Run this command from within a checked out copy of the kubewg repository
$ go run ./cmd/mkconf \
    --peer-name <peer-name> \
    --peer-namespace <peer-namespace> \
    --private-key "$(cat privatekey)" | qrencode -t ansiutf8
```
