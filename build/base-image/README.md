# Base Image

This Dockerfile is used to make base-image which is mainly used by agent, connector, cloud-agent.  Base image contains iptables/ip6tables, ipvsadm, ipset and iptables-wrapper, these are all needed by componets metioned before. 

[iptables-wrapper](https://github.com/kubernetes-sigs/iptables-wrappers) is the main reason for base-image, it can detect iptables version on host machine during runtime and change links to corresponding implementation in container, this is important, without it, those components might not create iptables rules correctly.