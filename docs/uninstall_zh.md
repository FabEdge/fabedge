## 卸载FabEdge



1. 使用helm删除主要资源

  ```shell
  $ helm uninstall fabedge -n fabedge
  ```

2. 删除其它资源

   ```shell
   $ kubectl -n fabedge delete cm --all
   $ kubectl -n fabedge delete pods --all
   $ kubectl -n fabedge delete secret --all
   $ kubectl -n fabedge delete job.batch --all
   ```
   
3. 删除namespace

   ```shell
   $ kubectl delete namespace fabedge
   ```

4. 删除所有边缘节点的`fabedge.conf`

   ```shell
   $ rm -f /etc/cni/net.d/fabedge.*
   ```

​    5. 删除所有节点的上fabedge相关的镜像

```shell
   $ docker images | grep fabedge | awk '{print $3}' | xargs -I{} docker rmi {}
```

 6.删除CustomResourceDefinition

```shell
$ kubectl delete CustomResourceDefinition "clusters.fabedge.io"
$ kubectl delete CustomResourceDefinition "communities.fabedge.io"
$ kubectl delete ClusterRole "fabedge-operator"
$ kubectl delete ClusterRoleBinding "fabedge-operator"
```

