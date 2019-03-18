# kubewg

kubewg is a Kubernetes controller that allows you to configure and manage
[Wireguard] VPN configuration using a Kubernetes API server.

It introduces the following [CustomResourceDefinition] resources:

* **Network**: Represents a Wireguard VPN network.
* **Peer**: Represents a single Peer in a a Network. Each peer will be
  allocated an address in the network's subnet.
* **RouteBinding**: Represents additional route configuration that should be
  used by all members of the VPN network.