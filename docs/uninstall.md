# Uninstall FabEdge

English | [中文](uninstall_zh.md)

1. Delete helm release

```
$ helm uninstall fabedge -n fabedge
```

2. Delete other resources

```
$ kubectl -n fabedge delete cm --all
$ kubectl -n fabedge delete pods --all
$ kubectl -n fabedge delete secret --all
$ kubectl -n fabedge delete job.batch --all
```

3. Delete namespace

```
$ kubectl delete namespace fabedge
```

4. Delete all FabeEdge configuration file from all edge nodes

```
$ rm -f /etc/cni/net.d/fabedge.*
```

5.  Delete all fabedge images on all nodes

```
   $ docker images | grep fabedge | awk '{print $3}' | xargs -I{} docker rmi {}
```

6. Delete CustomResourceDefinition

```
$ kubectl delete CustomResourceDefinition "clusters.fabedge.io"
$ kubectl delete CustomResourceDefinition "communities.fabedge.io"
$ kubectl delete ClusterRole "fabedge-operator"
$ kubectl delete ClusterRoleBinding "fabedge-operator"
```

