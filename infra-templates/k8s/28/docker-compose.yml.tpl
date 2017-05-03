kubelet:
    labels:
        io.rancher.container.dns: "true"
        io.rancher.container.create_agent: "true"
        io.rancher.container.agent.role: environmentAdmin
        io.rancher.scheduler.global: "true"
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: compute=true
        {{- end }}
    command:
        - kubelet
        - --kubeconfig=/etc/kubernetes/ssl/kubeconfig
        - --api_servers=https://kubernetes.kubernetes.rancher.internal:6443
        - --allow-privileged=true
        - --register-node=true
        - --cloud-provider=${CLOUD_PROVIDER}
        - --healthz-bind-address=0.0.0.0
        - --cluster-dns=10.43.0.10
        - --cluster-domain=cluster.local
        - --network-plugin=cni
        - --network-plugin-dir=/etc/cni/managed.d
        {{- if and (ne .Values.REGISTRY "") (ne .Values.POD_INFRA_CONTAINER_IMAGE "") }}
        - --pod-infra-container-image=${REGISTRY}/${POD_INFRA_CONTAINER_IMAGE}
        {{- else if (ne .Values.POD_INFRA_CONTAINER_IMAGE "") }}
        - --pod-infra-container-image=${POD_INFRA_CONTAINER_IMAGE}
        {{- end }}
    image: rancher/k8s:v1.5.4-rancher1-4
    volumes:
        - /run:/run
        - /var/run:/var/run
        - /sys:/sys:ro
        - /var/lib/docker:/var/lib/docker
        - /var/lib/kubelet:/var/lib/kubelet:shared
        - /var/log/containers:/var/log/containers
        - rancher-cni-driver:/etc/cni:ro
        - rancher-cni-driver:/opt/cni:ro
        - /dev:/host/dev
    net: host
    pid: host
    ipc: host
    privileged: true
    links:
        - kubernetes

proxy:
    labels:
        io.rancher.container.dns: "true"
        io.rancher.scheduler.global: "true"
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: compute=true
        {{- end }}
    command:
        - kube-proxy
        - --master=http://kubernetes.kubernetes.rancher.internal
        - --v=2
        - --healthz-bind-address=0.0.0.0
    image: rancher/k8s:v1.5.4-rancher1-4
    privileged: true
    net: host
    links:
        - kubernetes

etcd-operator:
    image: llparse/etcd-operator:dev
    command:
    - --debug=true
    - rancher
    - operator
    - --analytics=true
    - --gc-interval=10m
    net: host
    labels:
        io.rancher.container.agent.role: environmentAdmin
        io.rancher.container.create_agent: "true"
        io.rancher.container.dns: 'true'
        io.rancher.container.pull_image: always
    stdin_open: true
    tty: true

etcdv3:
    image: rancher/none
    net: none
    labels:
        io.rancher.operator: etcd
        io.rancher.operator.etcd.size: '3'
        io.rancher.operator.etcd.version: 3.1.5
        io.rancher.operator.etcd.paused: 'false'
        io.rancher.operator.etcd.antiaffinity: 'true'
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.operator.etcd.nodeselector: etcd=true
        {{- end }}
        io.rancher.operator.etcd.network: 'host'
        # io.rancher.operator.etcd.backup: '${ENABLE_BACKUPS}'
        # io.rancher.operator.etcd.backup.interval: '${BACKUP_INTERVAL}'
        # io.rancher.operator.etcd.backup.count: '${BACKUP_COUNT}'
        # io.rancher.operator.etcd.backup.delete: '${CLEANUP_BACKUPS_ON_DELETE}'
        # io.rancher.operator.etcd.backup.storage.type: '${STORAGE_TYPE}'
        # io.rancher.operator.etcd.backup.storage.driver: '${STORAGE_DRIVER}'
        io.rancher.service.selector.container: T3J6EntxHCTzVp=a9vHZuWXvFu6ma

etcd:
    image: rancher/etcd:v2.3.7-11
    labels:
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: etcd=true
        {{- end }}
        io.rancher.scheduler.affinity:container_label_ne: io.rancher.stack_service.name=$${stack_name}/$${service_name}
        io.rancher.sidekicks: data
    environment:
        RANCHER_DEBUG: 'true'
        EMBEDDED_BACKUPS: '${EMBEDDED_BACKUPS}'
        BACKUP_PERIOD: '${BACKUP_PERIOD}'
        BACKUP_RETENTION: '${BACKUP_RETENTION}'
        ETCD_HEARTBEAT_INTERVAL: '${ETCD_HEARTBEAT_INTERVAL}'
        ETCD_ELECTION_TIMEOUT: '${ETCD_ELECTION_TIMEOUT}'
    volumes:
    - etcd:/pdata
    - /var/etcd/backups:/data-backup
    volumes_from:
    - data

data:
    image: busybox
    entrypoint: /bin/true
    net: none
    volumes:
    - /data
    labels:
        io.rancher.container.start_once: 'true'

kubernetes:
    labels:
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: orchestration=true
        {{- end }}
        io.rancher.container.create_agent: "true"
        io.rancher.container.agent.role: environmentAdmin
        io.rancher.sidekicks: kube-hostname-updater
    command:
        - kube-apiserver
        - --service-cluster-ip-range=10.43.0.0/16
        - --etcd-servers=http://etcdv3.kubernetes.rancher.internal:2379
        - --insecure-bind-address=0.0.0.0
        - --insecure-port=80
        - --cloud-provider=${CLOUD_PROVIDER}
        - --allow_privileged=true
        - --admission-control=NamespaceLifecycle,LimitRanger,SecurityContextDeny,ResourceQuota,ServiceAccount
        - --client-ca-file=/etc/kubernetes/ssl/ca.pem
        - --tls-cert-file=/etc/kubernetes/ssl/cert.pem
        - --tls-private-key-file=/etc/kubernetes/ssl/key.pem
        - --runtime-config=batch/v2alpha1
    environment:
        KUBERNETES_URL: https://kubernetes.kubernetes.rancher.internal:6443
    image: rancher/k8s:v1.5.4-rancher1-4
    links:
        - etcd

kube-hostname-updater:
    net: container:kubernetes
    command:
        - etc-host-updater
    image: rancher/etc-host-updater:v0.0.2
    links:
        - kubernetes

kubectld:
    labels:
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: orchestration=true
        {{- end }}
        io.rancher.k8s.kubectld: "true"
        io.rancher.container.create_agent: "true"
        io.rancher.container.agent_service.kubernetes_stack: "true"
    environment:
        SERVER: http://kubernetes.kubernetes.rancher.internal
        LISTEN: ":8091"
    image: rancher/kubectld:v0.5.5
    links:
        - kubernetes

scheduler:
    command:
        - kube-scheduler
        - --master=http://kubernetes.kubernetes.rancher.internal
        - --address=0.0.0.0
    image: rancher/k8s:v1.5.4-rancher1-4
    {{- if eq .Values.CONSTRAINT_TYPE "required" }}
    labels:
        io.rancher.scheduler.affinity:host_label: orchestration=true
        {{- end }}
    links:
        - kubernetes

controller-manager:
    command:
        - kube-controller-manager
        - --master=https://kubernetes.kubernetes.rancher.internal:6443
        - --cloud-provider=${CLOUD_PROVIDER}
        - --address=0.0.0.0
        - --kubeconfig=/etc/kubernetes/ssl/kubeconfig
        - --root-ca-file=/etc/kubernetes/ssl/ca.pem
        - --service-account-private-key-file=/etc/kubernetes/ssl/key.pem
    image: rancher/k8s:v1.5.4-rancher1-4
    labels:
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: orchestration=true
        {{- end }}
        io.rancher.container.create_agent: "true"
        io.rancher.container.agent.role: environmentAdmin
    links:
        - kubernetes

rancher-kubernetes-agent:
    labels:
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: orchestration=true
        {{- end }}
        io.rancher.container.create_agent: "true"
        io.rancher.container.agent_service.labels_provider: "true"
    environment:
        KUBERNETES_URL: http://kubernetes.kubernetes.rancher.internal
    image: rancher/kubernetes-agent:v0.5.4
    privileged: true
    volumes:
        - /var/run/docker.sock:/var/run/docker.sock
    links:
        - kubernetes

rancher-ingress-controller:
    image: rancher/lb-service-rancher:v0.6.1
    labels:
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: orchestration=true
        {{- end }}
        io.rancher.container.create_agent: "true"
        io.rancher.container.agent.role: environment
    environment:
        KUBERNETES_URL: http://kubernetes.kubernetes.rancher.internal
    command:
        - lb-controller
        - --controller=kubernetes
        - --provider=rancher
    links:
        - kubernetes

addon-starter:
    image: rancher/k8s:v1.5.4-rancher1-4
    labels:
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: orchestration=true
        {{- end }}
        io.rancher.container.create_agent: 'true'
        io.rancher.container.agent.role: environmentAdmin
    environment:
        KUBERNETES_URL: https://kubernetes.kubernetes.rancher.internal:6443
        DISABLE_ADDONS: ${DISABLE_ADDONS}
        REGISTRY: ${REGISTRY}
        INFLUXDB_HOST_PATH: ${INFLUXDB_HOST_PATH}
    command:
        - addons-update.sh
    links:
        - kubernetes
