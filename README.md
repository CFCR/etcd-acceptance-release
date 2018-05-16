# CFCR etcd-acceptance-tests

## Deploy bosh-lite
```
blup up # deploy bosh lite
source ~/workspace/deployments/bosh-lite/bosh-env
```

## Deploy turbulence release

```
git clone https://github.com/cppforlife/turbulence-release.git
cd turbulence-release/
bosh cr --force && bosh ur
export TURB_{variables}
bosh deploy -d turbulence  manifests/example.yml --vars-env=TURB  --no-redact
```

## Deploy cfcr-etcd

```
bosh deploy -d etcd manifests/etcd.yml -o manifests/ops-files/local-release.yml \
  -o ~/workspace/kubo-ci/turbulence-agent.yml -o manifests/ops-files/share-links.yml
```

## Deploy etcd-acceptance-tests

```
bosh cr --force && bosh ur
bosh deploy -d etcd-acceptance deployment/etcd-acceptance.yml --vars ....
bosh run-errand read-availability-during-network-partition -d etcd-acceptance --keep-alive
```


## [Optionally] Make the tests locally

```
bosh  -d etcd-acceptance  scp -r  :/var/vcap/jobs/read-availability-during-network-partition/config/ src/acceptance/test-config
cd src/acceptance/
cat test-config/config.json #make sure the dns entries are resolvable locally
ginkgo -v --race --focus "Experiment Two" --  --config test-config/config.json
```
