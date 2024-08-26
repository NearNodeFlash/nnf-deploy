# CRD Version Bumper

Bump the CRD version, adding conversion webhooks and tests.

See Kubebuilder's [Tutorial: Multi-Version API](https://book.kubebuilder.io/multiversion-tutorial/tutorial) for a description of the mechanism. For more detail read the Kubernetes document [Versions in CustomResourceDefinitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/).

## Hub and Spokes

This tool implements the hub-and-spoke model of CRD versioning. See Kubebuilder's [Hubs, spokes, and other wheel metaphors](https://book.kubebuilder.io/multiversion-tutorial/conversion-concepts) for a description of the model.

The new CRD version will be the new hub, and the previous hub will become a new spoke. (Note: in the Kubebuilder book example, the old version is the hub and never changes, and all new versions are spokes.)

## Using CRD Bumper

Create the environment prior to running the tool:

```console
$ python3 -m venv venv
$ . venv/bin/activate
(venv) $ pip install -r requirements.txt
```

### Run the Tool

Clone a fresh copy of the repository that contains the CRDs and controllers, checking out to the default branch (master or main). The tool expects a repository that is compatible with kubebuilder and will use the `./PROJECT` file that is maintained by kubebuilder.

The following example will create a new API version `v1beta2` for the lustre-fs-operator repository, where `v1beta1` is the existing hub and `v1alpha1` is the most recent existing spoke. It begins by creating a new branch off "master" named `api-v1beta2`, where it will do all of its work.

```console
REPO=git@github.com:NearNodeFlash/lustre-fs-operator.git
crd-bumper.py --repo $REPO --most-recent-spoke v1alpha1 --prev-ver v1beta1 --new-ver v1beta2 all
```

The repository with its new API will be found under a directory named `workingspace/lustre-fs-operator`.

The new `api-v1beta2` branch will have a series of commits showing a progression of steps. Some of these commit messages will have an **ACTION** comment describing something that must be manually verified, and possibly adjusted, before the tests will succeed.

### Verification

After the entire progression of steps has completed, verify the results by running `make vet`, and paying attention to any of the **ACTION** comments described above. Once `make vet` is clean, move to the next debug step by running `make test`.

Do not run `make vet` or `make test` before the entire progression of steps has completed. **The individual commits do not build--the whole set of commits is required.**

### Stepping

Sometimes it can be helpful to do the steps one at a time. If the first step has not yet been done, then begin by using the `step` command in place of the `all` command. It begins, as with the `all` command, by creating a new branch off `master` named `api-v1beta2`, where it will do all of its work.

```console
crd-bumper.py --repo $REPO --most-recent-spoke v1alpha1 --prev-ver v1beta1 --new-ver v1beta2 step
```

Follow that with more steps, telling the tool to continue in the current branch that was created during the first step by adding `--this-branch`. The other args **must** remain the same as they were on the first command. The tool knows when there are no more steps to be done.

```console
crd-bumper.py --repo $REPO --most-recent-spoke v1alpha1 --prev-ver v1beta1 --new-ver v1beta2 --this-branch step
```

Do not attempt to run `make vet` or `make test` between steps. The individual commits do not build.

## Library and Tool Support

The library and tool support is taken from the [Cluster API](https://github.com/kubernetes-sigs/cluster-api) project. See [release v1.6.6](https://github.com/kubernetes-sigs/cluster-api/tree/release-1.6) for a version that contains multi-version support for CRDs where they have a hub with one spoke. (Note: In v1.7.0 they removed the old API--the old spoke--and their repo contains only one version, the hub.)

In the "References" section you'll find a link to a video from the Cluster API team where they go through an example of changing an API and providing conversion routines and tests for the change.

### Lossless Conversions

The libraries from the Cluster API project use an annotation on the spoke version of a resource to contain a serialized copy of the hub version of that resource. The libraries and tests use this to avoid data loss during hub->spoke->hub conversions.

## Code Markers

This tool uses "marker comments" in the Go code to indicate where generated code should be placed.
These markers are used the same way `controller-gen` uses code markers, as documented in Kubebuilder's [Markers for Config/Code Generation](https://book.kubebuilder.io/reference/markers).

The following markers must be present in the code prior to using this tool:

**+crdbumper:scaffold:builder**

Used in `internal/controller/suite_test.go` to indicate where additional calls to `SetupWebhookWithManager` should be placed. This should be in the `BeforeSuite()` function before the first reconciler is set up.

**+crdbumper:scaffold:gvk**

Used in `github/cluster-api/util/conversion/conversion_test.go` to indicate where additional `schema.GroupVersionKind{}` types should be placed.

**+crdbumper:scaffold:marshaldata**

Used in `github/cluster-api/util/conversion/conversion_test.go` to indicate where additional marshalling tests should be placed. This should be the last statement in the `TestMarshalData()` function.

**+crdbumper:scaffold:unmarshaldata**

Used in `github/cluster-api/util/conversion/conversion_test.go` to indicate where additional unmarshalling tests should be placed. This should be the last statement in the `TestUnmarshalData()` function.

**+crdbumper:scaffold:webhooksuitetest**

Used in `internal/controller/conversion_test.go` to indicate where additional tests for a new Kind may be placed. This should be the last statement within the main `Describe()` block.

**+crdbumper:scaffold:spoketest="GROUP.KIND"**

Used in `internal/controller/conversion_test.go` to indicate where additional spoke conversion tests should be placed. Each kind will have its own `Context()` block within the main `Describe()`, and this should be the last statement within each of those Context blocks. Replace "GROUP.KIND" with the API's group and kind, using the same spelling and use of lower/upper case letters as found in the `./PROJECT` file.

## References

### Kubernetes

[Versions in CustomResourceDefinitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/)

### Kubebuilder

[Tutorial: Multi-Version API](https://book.kubebuilder.io/multiversion-tutorial/tutorial)

[Hubs, spokes, and other wheel metaphors](https://book.kubebuilder.io/multiversion-tutorial/conversion-concepts)

### Cluster API

[Cluster API release 1.6 branch](https://github.com/kubernetes-sigs/cluster-api/tree/release-1.6)

Video: [SIG Cluster Lifecycle - ClusterAPI - API conversion code walkthrough](https://www.youtube.com/watch?v=Mk14N4SelNk)
