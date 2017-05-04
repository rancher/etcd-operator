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
        io.rancher.operator.etcd.restore.from: etcd
        io.rancher.operator.etcd.backup: 'true'
        io.rancher.operator.etcd.backup.interval: 1m
        io.rancher.operator.etcd.backup.count: '60'
        io.rancher.operator.etcd.backup.delete: 'false'
        io.rancher.operator.etcd.backup.storage.type: PersistentVolume
        io.rancher.operator.etcd.backup.storage.driver: local
        io.rancher.service.selector.container: app=etcd,cluster=${service_id}

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
