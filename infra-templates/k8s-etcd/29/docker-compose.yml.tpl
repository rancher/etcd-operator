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
        io.rancher.service.selector.container: app=etcd,cluster=${service_id}
