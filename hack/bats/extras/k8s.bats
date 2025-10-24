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
    for ((i=0; i<nodes; i++)); do
        limactl delete --force "${NAME}-$i" || :
        limactl start --tty=false --name "${NAME}-$i" "template:${TEMPLATE}" 3>&- 4>&-
        # NOTE: No support for multi-node clusters yet.
    done
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

# TODO: add a test for multi-node