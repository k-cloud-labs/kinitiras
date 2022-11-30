# kinitiras
![kinitiras-logo](docs/images/kinitiras.png)

[![Build Status](https://github.com/k-cloud-labs/kinitiras/actions/workflows/ci.yml/badge.svg)](https://github.com/k-cloud-labs/kinitiras/actions?query=workflow%3Abuild)
[![codecov](https://codecov.io/gh/k-cloud-labs/kinitiras/branch/main/graph/badge.svg?token=74uYpOiawR)](https://codecov.io/gh/k-cloud-labs/kinitiras)
[![Go Report Card](https://goreportcard.com/badge/github.com/k-cloud-labs/kinitiras)](https://goreportcard.com/report/github.com/k-cloud-labs/kinitiras)
[![Go doc](https://img.shields.io/badge/go.dev-reference-brightgreen?logo=go&logoColor=white&style=flat)](https://pkg.go.dev/github.com/k-cloud-labs/kinitiras)

[[中文](README-zh.md)]

A **lightweight** but **powerful** and **programmable** rule engine for kubernetes admission webhook.

If you want to use it in clientside with client-go, please use [pidalio](https://github.com/k-cloud-labs/pidalio).

## Quick Start

### Apply crd files to your cluster
```shell
kubectl apply -f https://raw.githubusercontent.com/k-cloud-labs/pkg/main/charts/_crds/bases/policy.kcloudlabs.io_overridepolicies.yaml
kubectl apply -f https://raw.githubusercontent.com/k-cloud-labs/pkg/main/charts/_crds/bases/policy.kcloudlabs.io_clusteroverridepolicies.yaml
kubectl apply -f https://raw.githubusercontent.com/k-cloud-labs/pkg/main/charts/_crds/bases/policy.kcloudlabs.io_clustervalidatepolicies.yaml
```

### Deploy webhook to cluster
All resources will be applied to `kinitiras-system` namespace by default. You can modify the deployment files as your expect.  

Pay attention to the deploy/webhook-configuration.yaml file. The default config will mutate and validate all kubernetes resources filtered by label `kinitiras.kcloudlabs.io/webhook: enabled`.  

**_YOU NEED TO UPDATE THE RULES AS YOUR EXPECT TO MINIMIZE THE EFFECTIVE SCOPE OF THE ADMISSION WEBHOOK._**  

After all changes done, just apply it to your cluster.  

```shell
kubectl apply -f deploy/
```

### Create policy 
Three kind of policy are supported.  

`OverridePolicy` is used to mutate object in the same namespace.  
`ClusterOverridePolicy` is used to mutate object in any namespace.  
`ClusterValidatePolciy` is used to validate object in any namespace.

For cluster scoped resource:
- Apply ClusterOverridePolicy by policies name in ascending;

For namespaced scoped resource, apply order is:
- First apply ClusterOverridePolicy;
- Then apply OverridePolicy;

Both mutate and validate policy are programmable via [CUE](https://cuelang.org/).   

### Constraint
1. The kubernetes object will be passed to CUE by `object` parameter.
2. The mutating result will be returned by `patches` parameter. 
3. The Validating result will be returned by `validate` parameter.  
4. Use `processing` to support data passing. It contains `http` and `output` schema.
   1. `http` used to make a http(s) request. Refer to: [http](https://pkg.go.dev/cuelang.org/go/pkg/tool/http) 
   2. `output` used to receive response. You should add some properties you need to it.

Schema:  

```cue
// for input parameter, oldObject only exist in `UPDATE` operation for clustervalidatepolicy 
object: _ @tag(object) 
oldObject: _ @tag(oldObject)

// use processing to pass data. A http reqeust will be make and output contains the response.
processing: {
	output: {
		// add what you need	
	}
	http: {
	    method: *"GET" | string
	    url: parameter.serviceURL
	    request: {
	    	body ?: bytes
	    	header: {}
	    	trailer: {}
	    }
	}
}

patch: {
	op: string
	path: string
	value: string
}

// for mutating result
patches: [...patch] 

// for validating result
validate: { 
	reason?: string
	valid: bool
}
```


## Examples
You can try some examples in the example folder.   

The `deletens-cvp.yaml` will protect the namespace labeled with `kinitiras.kcloudlabs.io/webhook=enabled` from being deleted.

The `addanno-op.yaml` will add annotation `added-by=op` to pod labeled with `kinitiras.kcloudlabs.io/webhook=enabled` in the default namespace.

The `addanno-cop.yaml` will add annotation `added-by=cue` to pod labeled with `kinitiras.kcloudlabs.io/webhook=enabled` in the default namespace.  

## Feature
- [x] Support mutate k8s resource by (Cluster)OverridePolicy via plaintext jsonpatch.
- [x] Support mutate k8s resource by (Cluster)OverridePolicy programmable via CUE.
- [x] Support validate k8s resource by ClusterValidatePolicy programmable via CUE.
- [x] Support Data passing by http request via CUE.
- [ ] kubectl plugin to validate CUE.
- [ ] ...

For more detail information for this project, please read the [roadmap](./ROADMAP.md).