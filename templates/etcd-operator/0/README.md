etcd-operator
=============

# Environment Mode

If no Rancher credentials are provided, operator will manage etcd clusters for the environment it belongs to.

# Experimental: Global Mode

Experimental, use at your own risk. Manage etcd clusters for an entire Rancher installation.

Note: The operator will needs direct network access to all etcd containers; currently only `host` network mode sort of works.