# FabEdge V1.0.0

Added:

1. Connector HA is implemented;
2. More calico modes are supported;
3. Flannel host-gw mode is supported;

Fixed:

1. Fix the bug that nodePort service doesn't work on cloud side;
2. Fix the bug that cloud-agent lost connections to connector after connector reboot;
3. Fix the bug that fabedge-agent can't initialize tunnels if strongswan container reboot;