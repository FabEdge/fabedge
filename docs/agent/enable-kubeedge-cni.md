# 启动kubeedge CNI功能

1. 关闭edgemesh

```yaml
edgeMesh:
    enable: false
```

2. 配置kubeedge cni配置:

```yaml
edged:
    enable: true
    cniBinDir: /opt/cni/bin
    cniCacheDirs: /var/lib/cni/cache
    cniConfDir: /etc/cni/net.d
    
    # 这一行默认配置文件是没有的，得自己添加  
    networkPluginName: cni
   
    networkPluginMTU: 1500
```

3. 安装CNI插件
从[https://github.com/containernetworking/plugins/releases](https://github.com/containernetworking/plugins/releases)
下载CNI插件，当前所用版本为0.9.1. 下载后解压，从里面复制`bridge`, `host-local`, `loopback`三个插件到边缘节点的`/opt/cni/bin`目录下.
CNI插件配置不需要用户配置，会由agent程序自动生成