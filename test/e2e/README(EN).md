# Instruction

```shell
[root@localhost e2e]# bash e2e.sh -h
USAGE:
  prepare-kubeconfig  [clusters_kubeconfig_store_dir] [cluster_ip_list_file_path]
                      e.g. prepare-kubeconfigs /tmp/e2ekubeconfigs ./cluster-master-ips
        
  multi-cluster       [clusters_kubeconfig_store_dir]
                      e.g. multi-cluster /tmp/e2ekubeconfigs
```

## 1.Testing e2e in a single cluster
The script executes the single-cluster test by default. Please execute the test on the master node of the cluster. The default timeout period is 300 secondsFor example, bash e2e.sh **200**.
```shell
bash e2e.sh
```
## <span id="j2">2.Prepare multiple cluster kubeconfig files</span>
**This script and the fabedge-e2e.test program can be executed on any cluster master node.**

```shell
bash e2e.sh prepare-kubeconfig /tmp/e2ekubeconfigs ./cluster-master-ips
```
>The script reads the master IP address list(one IP address in each row) of each cluster in the `./cluster-master-ips` file , obtains the `/root/.kube/config` file from the master node in the cluster in SCP mode, and saves the file to the directory using the corresponding master IP address, for example: `scp root@10.20.8.20:/root/.kube/config /tmp/e2ekubeconfigs/10.20.8.20`
>
>This option only prepares `kubeconfig` files for multiple cluster tests. You can manually collect each cluster `kubeconfig` file into a temporary directory.
- Option：prepare-kubeconfig.
- Cluster kubeconfig file diretory：/tmp/e2ekubeconfigs (Save the **temporary directory** to the configuration files of at least two cluster master nodes, **including the host cluster**).
- IP address list file of the primary node of the cluster：./cluster-master-ips (Save a text file with the IP addresses of at least two primary nodes of the cluster, **including host cluster**).

    ```shell
    # The IP address of the primary cluster node is independently wtitten to the file.
    [root@localhost e2e]# cat ./cluster-master-ips 
    10.20.8.20
    10.20.8.4
    10.20.8.12
    
    # The kubeconfig file named after the IP address of the corresponding cluster master node is saved to a temporary directory.
    [root@localhost e2e]# ls -hl /tmp/e2ekubeconfigs/
    -rw-------. 1 root root 5.6K Nov 29 00:38 10.20.8.4
    -rw-------. 1 root root 5.5K Nov 29 00:38 10.20.8.12
    -rw-------. 1 root root 5.5K Nov 29 00:38 10.20.8.20
    ```

## 3.Multi-cluster e2e testing
This script and `fabedge-e2e.test` can be executed on any cluster master node. The default timeout is 300 seconds.  For example, bash e2e.sh multi-cluster /tmp/e2ekubeconfigs **200**.
```shell
bash e2e.sh multi-cluster /tmp/e2ekubeconfigs
```
- Option：multi-cluster
- Cluster kubeconfig file diretory：/tmp/e2ekubeconfigs (Save the **temporary directory** to the configuration files of at least two cluster master nodes, **including the host cluster**. You can reference [Prepare multiple cluster kubeconfig files](#j2), then run the multi-cluster e2e testing).
