# FabricEdge Agent详细设计

## 概述

Agent负责以下功能

  * 维护CNI配置文件
  * 网络隧道维护(如: IPSec)
  * 维护iptables信息

因为边缘的不稳定性，导致其IP地址可能变动，而IP池也可能耗尽，为了保证隧道的可用性以及IP池的动态扩展，Agent需要动态地维护CNI配置文件和隧道配置文件。

为了动态地维护这些配置文件，Agent需要动态地获取配置信息，目前Agent是以Pod形式运行在边缘节点，但使用主机网络，避免对CNI插件的依赖。配置信息由一个ConfigMap挂载在这个Pod的特定目录，Agent
监听文件的变动，一旦有configmap变动，就重新生成上述配置文件。 

除了配置文件外， 如果Agent所在节点IP发生变动，Agent也要重新生成配置文件.

## Agent 配置文件格式
agent配置文件分两部分：

* agent所在节点的网络信息(见示例开头)
* 需要跟agent通信的节点的网络信息(见示例peers)

```yaml
id: C=CN, O=StrongSwan, CN=edge2
name: edge2
ip: 10.20.8.4
subnets:
  - 2.16.48.192/26
peers:
  - id: C=CN, O=StrongSwan, CN=node1
    name: node1
    ip: 10.20.8.169
    subnets:
      - 10.233.0.0/16
  - id: C=CN, O=StrongSwan, CN=edge3
    name: edge3
    ip: 10.20.8.12
    subnets:
      - 2.115.203.192/26
```

## 网络隧道维护

以IPSec实现strongswan为例，为了便于控制，agent会通过`vici`协议与strongswan通信，配置隧道, 配置隧道内容大致如下:

```
root@edge2:~# swanctl --list-conns
net-cloud: IKEv1/2, no reauthentication, rekeying every 14400s
  local:  10.20.8.4
  remote: 10.20.8.141
  local public key authentication:
    id: C=CN, O=StrongSwan, CN=edge2
    certs: C=CN, O=StrongSwan, CN=edge2
  remote public key authentication:
    id: C=CN, O=StrongSwan, CN=node1
  net-cloud-child: TUNNEL, rekeying every 3600s
    local:  2.16.48.192/26
    remote: 10.233.0.0/16
```

以上内容对应的IPsec配置如下：
```
conn net-cloud 
          left=10.10.8.4
          leftsubnet=10.10.0.0/16,3ffe:ffff:0:01ff::/64
          leftcert=edge2Cert.pem
          right=10.10.0.1
          rightsubnet=10.10.0.0/16,4000:eeee:0:01ff::/64
          rightid="C=CN, O=StrongSwan, CN=node1"
          auto=start
```

## CNI配置文件

以CNI项目组里的[bridge](https://www.cni.dev/plugins/current/main/bridge/)和[ipam](https://www.cni.dev/plugins/current/ipam/host-local/)插件配置为例

```json
{
  "cniVersion": "0.3.1",
  "name": "fabedge",
  "type": "bridge",
  "bridge": "br-fabedge",
  "isGateway": false,
  "isDefaultGateway": true,
  "forceAddress": true,
  "ipam": {
    "type": "host-local",
    "ranges": [
      [
        {
          "subnet": "2.16.48.192/26"
        }
      ]
    ]
  }
}
```
后面会根据需要来决定是否实现定制的CNI插件

## 维护iptables

为了确保网络数据可以转发到Pod里，需要对iptables进行配置:

* 创建`FABEDGE`链
* 将`FABEDGE` 链添加到`FORWARD`链
* 为每个edgeSubnet添加过滤规则

下面是`10.10.0.0/16`为例生成的iptables信息:

```
-N FABEDGE
-A FORWARD -j FABEDGE
-A FABEDGE -s 2.16.48.192/26 -j ACCEPT
-A FABEDGE -d 2.16.48.192/26 -j ACCEPT
```