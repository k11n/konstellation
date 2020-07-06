---
title: Linked Service Account
---

Linked service accounts allow apps to automatically assume identity of a specified IAM role. This functionality is built on top of a recent [AWS capability](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts-technical-overview.html) that allows IAM roles to be used for Kubernetes service accounts.

Typically, setting it up requires a few manual steps and can be challenging to reproduce correctly across clusters. In order to simplify the definition, Konstellation automates the setup of cluster-specific IAM roles and captures the definition in a kube-native LinkedServiceAccount resource. When you update this resource, Konstellation will sync those changes to the IAM role that it manages.

To use this, you need to first identify IAM Policies that defines the permissions that the app needs. This could be the global policies that AWS provides, or custom policies that you create.

:::info AWS SDK required
This feature requires your app to be using on of the [compatible AWS SDKs](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts-minimum-sdk.html)
:::

### Example

1. Create the service account

```
kon account create --account myaccount
```

2. Fill in the Policy ARNs

```yaml title="myaccount.yaml"
apiVersion: k11n.dev/v1alpha1
kind: LinkedServiceAccount
metadata:
  name: myaccount
spec:
  aws:
    policyArns:
    - arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess
```

In this example, using one of the global policies, `myaccount` will have read-only access to all buckets in S3.

3. References in app manifest

```
kon app edit myapp
```

```yaml title="app.yaml"
...
spec:
  serviceAccount: myaccount
```

The app will be re-deployed to assume the managed IAM role.

### Managing accounts

The CLI provides other management functions of the linked account. For example, if you need to change the policies that are attached for the account, use the `kon account edit` command.

Any changes will be synced back to IAM.
