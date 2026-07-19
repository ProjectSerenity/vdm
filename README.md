# VDM: Vendoring Dependency Manager

VDM is a simple tool for managing external dependencies. VDM uses a dependency manifest written in the VDM Encoding Scheme and patch files to download the dependencies into a source repository.

VDM works with a dependencies manifest called `deps.vdm`. This uses a simple syntax to express the set of dependencies.

Commands:

- The dependencies specified in `deps.vdm` are installed in the source repo using `vdm vendor`.
- The dependency manifests are updated with newer minor/patch versions using `vdm update`. Note that this will not affect the source repository without subsequently calling `vdm vendor`.
- The dependency graph can be checked for vulnerabilities and analysed for unused dependencies using `vdm check ROOTS`. The given roots are one or more Bazel path.
  - As an example, the VDM repository is checked using `vdm check //:vdm //cmd/... //internal/...`.
