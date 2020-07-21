module github.com/b01901143/secret-sync-controller

go 1.13

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible
	k8s.io/api => k8s.io/api v0.17.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.3
	k8s.io/client-go => k8s.io/client-go v0.17.3
)

require (
	cloud.google.com/go v0.60.0
	gonum.org/v1/plot v0.7.0
	google.golang.org/genproto v0.0.0-20200709005830-7a2ca40e9dc3
	google.golang.org/grpc v1.30.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/test-infra v0.0.0-20200709040651-6563d6a195ee
	sigs.k8s.io/kind v0.8.1 // indirect
)
