# kinitiras
![kinitiras-logo](docs/images/kinitiras.png)

[![Build Status](https://github.com/k-cloud-labs/kinitiras/actions/workflows/ci.yml/badge.svg)](https://github.com/k-cloud-labs/kinitiras/actions?query=workflow%3Abuild)
[![codecov](https://codecov.io/gh/k-cloud-labs/kinitiras/branch/main/graph/badge.svg?token=74uYpOiawR)](https://codecov.io/gh/k-cloud-labs/kinitiras)
[![Go Report Card](https://goreportcard.com/badge/github.com/k-cloud-labs/kinitiras)](https://goreportcard.com/report/github.com/k-cloud-labs/kinitiras)
[![Go doc](https://img.shields.io/badge/go.dev-reference-brightgreen?logo=go&logoColor=white&style=flat)](https://pkg.go.dev/github.com/k-cloud-labs/kinitiras)

[[English](README.md)]

**轻量**、**功能强大**、**可编程的** k8s admission webhook 规则引擎。

如果你想在客户端实现类似能力，请使用 [pidalio](https://github.com/k-cloud-labs/pidalio)。

## 快速开始

### 部署 CRD
```shell
kubectl apply -f https://raw.githubusercontent.com/k-cloud-labs/pkg/main/charts/_crds/bases/policy.kcloudlabs.io_overridepolicies.yaml
kubectl apply -f https://raw.githubusercontent.com/k-cloud-labs/pkg/main/charts/_crds/bases/policy.kcloudlabs.io_clusteroverridepolicies.yaml
kubectl apply -f https://raw.githubusercontent.com/k-cloud-labs/pkg/main/charts/_crds/bases/policy.kcloudlabs.io_clustervalidatepolicies.yaml
```

### 部署应用
所有资源将会被默认部署在 `kinitiras-system` 命名空间下，你可以按需修改部署文件 `deploy/deploy.yaml`。

默认 webhook 配置会对所有包含 `kinitiras.kcloudlabs.io/webhook: enabled` 标签的资源对象进行拦截，你可以按需修改对应文件 `deploy/webhook-configuration.yaml`。
**_部署前请按需修改所有 `deploy` 下的部署文件._**

修改完之后执行如下命令部署到集群即可。

```shell
kubectl apply -f deploy/
```

### 创建策略
支持三种策略，作用和生效范围如下：

`OverridePolicy` 可以修改同命名空间下的资源对象。
`ClusterOverridePolicy` 可以修改任意命名空间下的资源对象。
`CLusterValidatePolciy` 可以校验任意命名空间下的资源对象的操作。

针对集群级别的资源:
- 按照匹配的 `ClusterOverridePolicy` 策略名称的字母顺序进行应用；

针对命名空间级别的资源对象:
- 首先应用所有匹配的 `ClusterOverridePolicy`;
- 其次应用虽有匹配的 `OverridePolicy`;

策略的可编程能力依赖 [CUE](https://cuelang.org/).

### 约束
1. K8s 资源对象通过 `object` 参数传递，针对修改请求，老资源对象将通过 `oldObject` 参数传递，无需入参时可省略，但参数名不可修改；
2. Mutating 结果将以 `patches` 参数返回；
3. Validating 结果将以 `validate` 参数返回； 
4. 数据传输在 `processing` 节点定义，包含 `http` 和 `output` 两个子节点
    1. `http` 用来发送 http(s) 请求. 参考: [http](https://pkg.go.dev/cuelang.org/go/pkg/tool/http)；
    2. `output` 用来接受返回结果，按需定义其结构即可；

结构定义:

```cue
// oldObject 只针对 `clustervalidatepolicy` 策略中的 `UPDATE` 操作  
object: _ @tag(object) 
oldObject: _ @tag(oldObject)

processing: {
	output: {
		// 按需自定义返回体结构	
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

// mutating 返回结构
patches: [...patch] 

// validating 返回结构
validate: { 
	reason?: string
	valid: bool
}
```


## 例子
example 文件夹下有如下实例供参考。

`deletens-cvp.yaml` 保护带有 `kinitiras.kcloudlabs.io/webhook=enabled` 标签的命名空间被删除。

`addanno-op.yaml` 将会给默认命名空间下带有 `kinitiras.kcloudlabs.io/webhook=enabled` 标签的 pod 添加 `added-by=op` annotation。

`addanno-cop.yaml` 将会给默认命名空间下带有 `kinitiras.kcloudlabs.io/webhook=enabled` 标签的 pod 添加 `added-by=cue` annotation。

## 特性
- [x] 支持通过在 (Cluster)OverridePolicy 策略中以 plaintext 方式实现对 k8s 资源对象的修改。
- [x] 支持通过在 (Cluster)OverridePolicy 策略中以 cue 可编程的方式实现对 k8s 资源对象的修改。
- [x] 支持通过在 ClusterValidatePolicy 策略中以 cue 可编程的方式实现对 k8s 资源对象的校验。
- [x] 支持在策略中使用 CUE 发送 http 请求。
- [ ] 支持使用 kubectl plugin 进行 CUE 内容校验。
- [ ] ...

更多详细内容，请参考 [roadmap](./ROADMAP.md)。