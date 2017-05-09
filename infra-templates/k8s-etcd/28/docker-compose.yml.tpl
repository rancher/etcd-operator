etcd:
    image: llparse/etcd:v3.0.17-beta.0
    labels:
        {{- if eq .Values.CONSTRAINT_TYPE "required" }}
        io.rancher.scheduler.affinity:host_label: etcd=true
        {{- end }}
        io.rancher.scheduler.affinity:container_label_ne: io.rancher.stack_service.name=$${stack_name}/$${service_name}
        io.rancher.sidekicks: data
        io.rancher.container.pull_image: always
    environment:
        RANCHER_DEBUG: 'true'
        EMBEDDED_BACKUPS: '${EMBEDDED_BACKUPS}'
        BACKUP_PERIOD: '${BACKUP_PERIOD}'
        BACKUP_RETENTION: '${BACKUP_RETENTION}'
        ETCD_HEARTBEAT_INTERVAL: '${ETCD_HEARTBEAT_INTERVAL}'
        ETCD_ELECTION_TIMEOUT: '${ETCD_ELECTION_TIMEOUT}'
        ETCD_MIGRATE: v3
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
