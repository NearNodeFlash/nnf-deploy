# CRD Version Bumper

Bump the CRD version, adding conversion webhooks and tests.

See Kubebuilder's [Tutorial: Multi-Version API](https://book.kubebuilder.io/multiversion-tutorial/tutorial) for a description of the mechanism. For more detail read the Kubernetes document [Versions in CustomResourceDefinitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/).

## Hub and Spokes

This tool implements the hub-and-spoke model of CRD versioning. See Kubebuilder's [Hubs, spokes, and other wheel metaphors](https://book.kubebuilder.io/multiversion-tutorial/conversion-concepts) for a description of the model.

## Using CRD Bumper

See [CRD Bumper](https://nearnodeflash.github.io/dev/repo-guides/crd-bumper/readme/) for documentation on using the `crd-bumper` tools.

See [Editing APIs](https://nearnodeflash.github.io/dev/repo-guides/crd-bumper/editing-apis/) for guidance on editing CRD APIs.
