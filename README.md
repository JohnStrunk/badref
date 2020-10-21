# badref

_A small utility to find bad
[`ownerReferences`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#ownerreference-v1-meta)
in a Kubernetes cluster_

The ownerReferences field in Kubernetes object metadata allows objects to be
automatically cleaned up when their "owner(s)" are deleted. However, [there are
specific rules about the owner/owned
relationship.](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/)
For example:

- Cross-namespace ownership is not allowed
- Namespaced objects cannot own a cluster-scoped object

When these rules are violated, the Kubernetes garbage collector may delete
resources unexpectedly. This utility scans all the objects in a cluster and
validates that the owner references are correct.

## Usage

```console
$ go get github.com/JohnStrunk/badref
go: downloading github.com/JohnStrunk/badref v0.0.0-20201021144329-e37e4c101b62
go: github.com/JohnStrunk/badref upgrade => v0.0.0-20201021144329-e37e4c101b62

$ badref --kubeconfig /path/to/kubeconfig
Discovered 263 resources
Scanned 252 objects
Checked 6 owner references
All ok!
```
