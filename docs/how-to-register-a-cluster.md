# 如何注册集群

多集群通信需要把各个集群的端点信息在主机群注册：


1. 在主集群创建一个cluster资源:

   ```yaml
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
   metadata:
     name: beijing
   ```

2. 查看token

   ```shell
   # kubectl get cluster beijing
   NAME   TOKEN   AGE
   beijing          eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MzcwODUzNTEsInN1YiI6ImJlaWppbmcifQ.X-1-o0fDG40b0cB4wExYCbeFke4WtqMQKnGa9u_2js_cSJibJrgzQiKeL7K4PiiYsRrmqYc2rQDA3e-qoNcgDSPO0h4m92MvGkrj5_lmkDaEydljfSlgrAc-xVIXl6-vzT42w9Atg7qWmAzWLk4oyQ3dg-aBvckpVD16SfpqEi-LLBS_ymSVleLTAHA5e77sQkbxu0WkllGDWX57Xr6pkX87r4QKEB7ynwwr75fKpeD5w99V-5l3OB-o4PI4ysqgsFhZNzVT7OMzDiXFGSFFu6FAMw7_zKj0pChW2b4D0gLLkcc-we7Rd597XAPviSnz1gIw18hx79MPkITlElB8o8pUDk1bZ5WP5kc2sFCsOfIFjo87s6wH9TXQyesccCCO38qzN5_iTJ1dj_AF9cbiwusbkXs93rJUObm_Rai33rPWNjL_LMvl-Lz_AknghVf8e2mcbh72d9drdAV2bmuBmVkGHCD40CksMytb2ePRpwDa4tWX6w96iIcC21NLcBOtQ_MNQp5KtKAnhnG3MtydlZSEUWWu1V4SVSi_K6c9UH6GAbjUdOdd3dezewczAVM6_xhylxY6gizW-yx--8sSzN2JSp2Uy2TYKQJbpNv9KI9zYIqqlxNegDj15ew33_PBwUyWZsIEc0zW4neK2E_YO8HSbZL8IIHsqfSLWBQJJ30   13m
   ```

   *注: token由fabedge-operator负责生成，该token有效期内使用该token进行成员集群初始化*

3. 在成员集群部署FabEdge，部署时使用第一步生成的token, 成员集群的operator会把本集群的connector信息上报至主机群。

   ```yaml
   # kubectl get cluster beijing -o yaml
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
     name: beijing
   spec:
     endPoints:
     - id: C=CN, O=fabedge.io, CN=beijing.connector
       name: beijing.connector
       nodeSubnets:
       - 10.20.8.12
       - 10.20.8.38
       publicAddresses:
       - 10.20.8.12
       subnets:
       - 10.233.0.0/18
       - 10.233.70.0/24
       - 10.233.90.0/24
       type: Connector
     token: eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MzcwODUzNTEsInN1YiI6ImJlaWppbmcifQ.X-1-o0fDG40b0cB4wExYCbeFke4WtqMQKnGa9u_2js_cSJibJrgzQiKeL7K4PiiYsRrmqYc2rQDA3e-qoNcgDSPO0h4m92MvGkrj5_lmkDaEydljfSlgrAc-xVIXl6-vzT42w9Atg7qWmAzWLk4oyQ3dg-aBvckpVD16SfpqEi-LLBS_ymSVleLTAHA5e77sQkbxu0WkllGDWX57Xr6pkX87r4QKEB7ynwwr75fKpeD5w99V-5l3OB-o4PI4ysqgsFhZNzVT7OMzDiXFGSFFu6FAMw7_zKj0pChW2b4D0gLLkcc-we7Rd597XAPviSnz1gIw18hx79MPkITlElB8o8pUDk1bZ5WP5kc2sFCsOfIFjo87s6wH9TXQyesccCCO38qzN5_iTJ1dj_AF9cbiwusbkXs93rJUObm_Rai33rPWNjL_LMvl-Lz_AknghVf8e2mcbh72d9drdAV2bmuBmVkGHCD40CksMytb2ePRpwDa4tWX6w96iIcC21NLcBOtQ_MNQp5KtKAnhnG3MtydlZSEUWWu1V4SVSi_K6c9UH6GAbjUdOdd3dezewczAVM6_xhylxY6gizW-yx--8sSzN2JSp2Uy2TYKQJbpNv9KI9zYIqqlxNegDj15ew33_PBwUyWZsIEc0zW4neK2E_YO8HSbZL8IIHsqfSLWBQJJ30
   ```

   