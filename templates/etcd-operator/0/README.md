etcd-operator
=============

## Environment Mode

Operator will manage etcd clusters for a specific environment.

Provide an `Environment` API Access Key, Secret Key and URL.

## Global Mode

**Experimental, use at your own risk.** Manage etcd clusters across all environments.

Provide an `Account` API Access Key, Secret Key and URL.

### Limitations
The operator container will need direct network access to all etcd containers. This may be difficult to achieve across environments.