# Network Service Mesh CSI Driver

A [Container Storage
Interface](https://github.com/container-storage-interface/spec/blob/master/spec.md)
driver for Kubernetes that facilitates injection of the Network Service API which is nominally served over a Unix Domain Socket created by [Network Service Manager](https://github.com/networkservicemesh/cmd-nsmgr) (NSMGR) DaemonSet. Therefore it is necessary to inject the Network Service API socket into each pod. The primary motivation for using a CSI driver for this purpose is to avoid the use of
[hostPath](https://kubernetes.io/docs/concepts/storage/volumes/#hostpath)
volumes in workload containers, which is commonly disallowed or limited by
policy due to inherent security concerns. Note that `hostPath` volumes are
still required for the CSI driver to interact with the
[Kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/)
(see [Limitations](#limitations)).

This driver mounts a directory containing a Network Service API socket as an ephemeral inline volume into workload pods.

## Envorinment config

* `NSM_NODE_NAME`       - Envvar from which to obtain the node ID
* `NSM_PLUGIN_NAME`     - Plugin name to register (default: "csi.networkservicemesh.io")
* `NSM_SOCKET_DIR`      - Path to the NSM API socket directory
* `NSM_CSI_SOCKET_PATH` - Path to the CSI socket (default: "/nsm-csi/csi.sock")
* `NSM_VERSION`         - Version (default: "undefined")

## How it Works

This component can be deployed as a sidecar for the NSMGR or a separate pod and registered with the kubelet using the official CSI Node Driver Registrar image. The NSM CSI Driver and the NSMGR share the directory hosting the Network Service API Unix Domain Socket using a `hostPath` volume. An `emptyDir` volume cannot be used since the backing directory would be removed if the NSM CSI Driver pod is restarted,invalidating the mount into workload containers.

When pods declare an ephemeral inline mount using this driver, the driver is invoked to mount the volume. The driver does a read-only bind mount of the directory containing the Network Service API Unix Domain Socket into the container at the requested target path.

Similarly, when the pod is destroyed, the driver is invoked and removes the
bind mount.

## Dependencies

CSI Ephemeral Inline Volumes require at least Kubernetes 1.15 (enabled via the `CSIInlineVolume` feature gate) or 1.16 (enabled by default).

## Limitations

CSI drivers are registered as plugins and interact with the Kubelet, which requires several `hostPath` volumes. As such, this driver cannot be used in environments where `hostPath` volumes are forbidden.

## Troubleshooting

This component has a fairly simple design and function but some of the
following problems may manifest.

### Failure to Register with the Kubelet

This problem can be diagnosed by dumping the logs of the kubelet (if possible), the driver registrar container, and the NSM CSI driver container. Likely suspects are a misconfiguratoin of the various volume mounts needed for communication between the register, the NSM CSI driver, and the kubelet.

### Failure to Mount the Socket Directory

This problem can be diagnosed by dumping the NSM CSI driver logs.

### Failure to Terminate Pods when Driver is Unhealthy Or Removed

If the NSM CSI Driver is removed (or is otherwise unhealthy), any pods that
contain a volume mounted by the driver will fail to fully terminate until
driver health is restored. The describe command (i.e. kubectl describe) will show the failure to unmount the volume. Kubernetes will continue to retry to unmount the volume via the CSI driver. Once the driver has been restored, the unmounting will eventually succeed and the pod will be fully terminated.

### Broken Mount when the CSI Driver Pod is Restarted

Ensure that the Network Service API socket directory is shared with the NSM CSI Driver via a `hostPath` volume. The directory backing `emptyDir` volumes are tied to the pod instance and invalidated when the pod is restarted.

## Acknowledgments

This application was developed based on [spiffe-csi](https://github.com/spiffe/spiffe-csi)
