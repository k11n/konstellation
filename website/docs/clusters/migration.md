---
title: Cluster Migration
---

The following is a guide to migrating from an existing Konstellation cluster to a new one. For the purpose of the guide, it uses Route53 as the example DNS provider.

1. Export a backup of Konstellation manifests

 ```
 kon cluster export --dir <export_dir>
 ```

1. Create a new cluster with desired settings

 ```
 % kon cluster create
 ```

 Choose the same VPC as the previous cluster to minimize descrepencies in the networking setup.

1. Select the new cluster to work with

 ```
 % kon cluster select <newcluster>
 ```

1. Sync SSL certs

 ```
 % kon certificate sync
 ```

1. Import configuration into new cluster

 ```
 % kon cluster import --dir <export_dir>
 ```

 This will import all of your existing apps, builds, configs, and linked accounts into the new cluster, creating the necessary IAM roles as defined by the linked accounts.

 Optional: If you've made other changes to the existing Kubernetes cluster, apply those changes to the new cluster as well.

1. Check your apps to ensure everything's running correctly

 ```
 % kon app list
 ```

 All apps should have Status `running` if they are deployed properly.

2. Get new load balancer address for the entrypoint app

 ```
 % kon app status <main_app>

 Target: production
 Hosts: www.myhost.com
 Load Balancer: ff833335-istiosystem-konin-xxxxxxx.us-west-2.elb.amazonaws.com
 ```

 and test to ensure that the route to your app is working

 ```
 %  host=<your host> \
 target=<load balancer address> \
 ip=$(dig +short x.example | head -n1) \
 curl -sv --resolve $host:443:$ip https://$host
 ```

1. Change the routing policy on the existing DNS entry to be `Weighted` in order to move traffic over to the new cluster gradually. If you are using IPv6, do the same for the AAAA entry.

 <img src="/img/screen/migration-1.png" alt="change routing policy" style={{width: "400px"}}/>

2. Create a new DNS entry with the same name, choose `Weighted` routing policy and give it a weight of 1. Hit create, and now ~1% of your site's traffic will be sent to the new cluster.

 <img src="/img/screen/migration-2.png" alt="add new DNS entry" style={{width: "400px"}}/>

3. Check traffic on Grafana and ensure things look right

 ```
 % kon launch grafana
 ```

 Navigate to the `App Overview` dashboard and select the app that should be receiving traffic. You should see traffic ramping up.

 ![grafana](/img/screen/migration-3.png)

1. Increase the weight of the new cluster gradually, to ensure that autoscalers would have a chance to catch up with getting the right number of instances of pods. Once traffic is fully shifted over to the new cluster, you can delete the DNS entries of the old cluster.

1. Destroy the existing cluster with `kon cluster destroy --cluster <old_cluster>`
