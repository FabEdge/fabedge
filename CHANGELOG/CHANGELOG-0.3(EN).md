# FabEdge V0.3

## New Feature

1. Flannel plug-in can be used for cloud clusters

   [Flannel](https://github.com/flannel-io/flannel) is simple, easy to use, there are a large number of users, this version added support for it. So far, FabEdge supports: Calico, Flannel.   

2. Support SuperEdge

   [SuperEdge](https://github.com/superedge/superedge/blob/main/README_CN.md) is the edge of Kubernetes native container plan, It extends Kubernetes' powerful container management capabilities to edge computing scenarios and provides solutions to common technical challenges in edge computing scenarios. This version of FabEdge adds support for SuperEdge.

3. Support OpenYurt

   [OpenYurt](https://openyurt.io/) is hosted under Cloud Native Computing Foundation (CNCF) [Sandbox Project](https://www.cncf.io/sandbox-projects/). It is built on native Kubernetes with the goal of extending Kubernetes to seamlessly support edge computing scenarios. FabEdge this version adds OpenYurt support.  

## Other updates

1. Automatic identification of cloud POD network segment

   Operator automatically identifies the POD network segment of the cloud cluster without manual input. 

2. User-defined edge node labels are supported

   Users can customize the set of labels used to identify edge nodes managed by FabEdge.  

