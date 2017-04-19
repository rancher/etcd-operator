version: '2'
services:
  etcd-operator:
    image: llparse/etcd-operator:dev
    command:
    - --color=${COLOR}
    - --debug=${DEBUG}
    - rancher
    - operator
    - --analytics=${ANALYTICS}
    - --gc-interval=${GC_INTERVAL}
    - --chaos-level=${CHAOS_LEVEL}
    environment:
      CATTLE_URL: ${CATTLE_URL}
      CATTLE_ACCESS_KEY: ${CATTLE_ACCESS_KEY}
      CATTLE_SECRET_KEY: ${CATTLE_SECRET_KEY}
    labels:
      io.rancher.container.dns: 'true'
      io.rancher.container.pull_image: always
    network_mode: host