# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# This test verifies that a Kubernetes cluster can be started and that the single node is ready.

load "../helpers/load"

: "${TEMPLATE:=k8s}"

# Instance names are "${NAME}-0", "${NAME}-1", ...
NAME="k8s"

get_num_nodes() {
    local nodes=0
    for tag in "${BATS_TEST_TAGS[@]}"; do
        if [[ $tag =~ ^nodes:([0-9]+)$ ]]; then
            nodes="${BASH_REMATCH[1]}"
        fi
    done
    if [[ $nodes -eq 0 ]]; then
        echo >&2 "nodes:N tag is required"
        exit 1
    fi
    echo "$nodes"
}

local_setup() {
    local nodes=$(get_num_nodes)
    local params=""
    for ((i=0; i<1; i++)); do
        limactl delete --force "${NAME}-$i" || :
        local limactl_start_flags="--tty=false --name "${NAME}-$i""
        # Multi-node setup requires user-v2 network for VM-to-VM communication
        if [[ $nodes -gt 1 ]]; then
            limactl_start_flags+=" --network lima:user-v2"
        fi
        limactl start ${limactl_start_flags} "template:${TEMPLATE}" 3>&- 4>&- &
    done
    wait $(jobs -p)
    # Multi-node setup
    if [[ $nodes -gt 1 ]]; then
        for ((i=0; i<nodes; i++)); do
            if [[ $i -eq 0 && "${TEMPLATE}" == "k8s" ]]; then
                # Get the join command from the first node
                join_command=$(limactl shell "${NAME}-0" sudo kubeadm token create --print-join-command)
                # kubeadm join ADDRESS --token TOKEN --discovery-token-ca-cert-hash DISCOVERY_TOKEN_CA_CERT_HASH
                read -ra words <<< "$join_command"
                assert_equal "${words[1]} ${words[3]} ${words[5]}" "join --token --discovery-token-ca-cert-hash"
                params=".param.url=\"https://${words[2]}\"|.param.token=\"${words[4]}\"|.param.discoveryTokenCaCertHash=\"${words[6]}\""
            elif [[ $i -eq 0 && "${TEMPLATE}" == "k3s" ]]; then
                url=$(printf "https://lima-%s.internal:6443\n" "${NAME}-0")
                token=$(limactl shell "${NAME}-0" sudo cat /var/lib/rancher/k3s/server/node-token)
                params=".param.url=\"${url}\"|.param.token=\"${token}\""
            else
                # Execute the join command on worker nodes
                limactl delete --force "${NAME}-$i" || :
                local limactl_start_flags="--tty=false --name "${NAME}-$i""
                limactl_start_flags+=" --network lima:user-v2 --set $params"
                limactl start ${limactl_start_flags} "template:${TEMPLATE}" 3>&- 4>&- &
            fi
        done
        wait $(jobs -p)
    fi
    for node in $(k get node -o name); do
	    k wait --timeout=5m --for=condition=ready "${node}"
    done
}

local_teardown() {
    local nodes=$(get_num_nodes)
    for ((i=0; i<nodes; i++)); do
        limactl delete --force "${NAME}-$i" || :
    done
}

k() {
    # The host home directory is not mounted in the case of k8s.
    limactl shell --workdir=/ "${NAME}-0" -- kubectl "$@"
}

# bats test_tags=nodes:1
@test 'Single-node' {
    # Deploy test services
    services=(nginx coredns)
    for svc in "${services[@]}"; do
        k create deployment "$svc" --image="${TEST_CONTAINER_IMAGES["$svc"]}"
    done
    for svc in "${services[@]}"; do
        k rollout status deployment "$svc" --timeout 60s
    done

    # Test TCP port forwarding
    k create service nodeport nginx --node-port=31080 --tcp=80:80
    run curl --fail --silent --show-error --retry 30 --retry-all-errors http://localhost:31080
    assert_success
    assert_output --partial "Welcome to nginx"

    # Test UDP port forwarding
    #
    # `kubectl create service nodeport` does not support UDP, so use `kubectl expose` instead.
    # https://github.com/kubernetes/kubernetes/issues/134732
    k expose deployment coredns --port=53 --type=NodePort \
        --overrides='{"spec":{"ports":[{"port":53,"protocol":"UDP","targetPort":53,"nodePort":32053}]}}'
    run dig @127.0.0.1 -p 32053 lima-vm.io
    assert_success

    # Cleanup
    for svc in "${services[@]}"; do
        k delete service "$svc"
        k delete deployment "$svc"
    done
}

# bats test_tags=nodes:3
@test 'Multi-node' {
    # Based on https://github.com/rootless-containers/usernetes/blob/gen2-v20250828.0/hack/test-smoke.sh
    k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: dnstest
  labels:
    run: dnstest
spec:
  type: ClusterIP
  clusterIP: None
  ports:
  - name: http
    protocol: TCP
    port: 80
    targetPort: 80
  selector:
    run: dnstest
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dnstest
spec:
  serviceName: dnstest
  selector:
    matchLabels:
      run: dnstest
  replicas: 3
  template:
    metadata:
      labels:
        run: dnstest
    spec:
      containers:
      - name: dnstest
        image: ${TEST_CONTAINER_IMAGES[nginx]}
        ports:
        - containerPort: 80
EOF
    k rollout status --timeout=5m statefulset/dnstest || {
        k describe pods -l run=dnstest
        false
    }
    # --rm requires -i
    k run -i --rm --image=${TEST_CONTAINER_IMAGES[nginx]} --restart=Never dnstest-shell -- sh -exc 'for f in $(seq 0 2); do wget -O- http://dnstest-${f}.dnstest.default.svc.cluster.local; done'
    k delete service dnstest
    k delete statefulset dnstest
}
