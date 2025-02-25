# Copyright 2022 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

go = [
    module(
        name = "github.com/bazelbuild/buildtools",
        version = "v0.0.0-20250130132114-635c1223b1e6",
        packages = [
            package(
                name = "github.com/bazelbuild/buildtools/build",
                build_file = "patches/github.com_bazelbuild_buildtools_build.BUILD",
            ),
            package(
                name = "github.com/bazelbuild/buildtools/labels",
            ),
            package(
                name = "github.com/bazelbuild/buildtools/tables",
                no_tests = True,  # The tests don't play nicely when vendored into another Bazel workspace.
            ),
            package(
                name = "github.com/bazelbuild/buildtools/testutils",
            ),
        ],
        patches = [
            "patches/github.com_bazelbuild_buildtools_build_build_defs.bzl",
        ],
        patch_args = ["-p1"],
    ),
    module(
        name = "github.com/google/go-cmp",
        version = "v0.6.0",
        packages = [
            package(
                name = "github.com/google/go-cmp/cmp",
                deps = [
                    "github.com/google/go-cmp/cmp/internal/diff",
                    "github.com/google/go-cmp/cmp/internal/flags",
                    "github.com/google/go-cmp/cmp/internal/function",
                    "github.com/google/go-cmp/cmp/internal/testprotos",
                    "github.com/google/go-cmp/cmp/internal/teststructs",
                    "github.com/google/go-cmp/cmp/internal/value",
                ],
                test_deps = [
                    "github.com/google/go-cmp/cmp/cmpopts",
                    "github.com/google/go-cmp/cmp/internal/teststructs/foo1",
                    "github.com/google/go-cmp/cmp/internal/teststructs/foo2",
                ],
            ),
            package(
                name = "github.com/google/go-cmp/cmp/cmpopts",
                deps = [
                    "github.com/google/go-cmp/cmp",
                    "github.com/google/go-cmp/cmp/internal/function",
                ],
                test_deps = [
                    "github.com/google/go-cmp/cmp/internal/flags",
                ],
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/diff",
                deps = [
                    "github.com/google/go-cmp/cmp/internal/flags",
                ],
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/flags",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/function",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/testprotos",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/teststructs",
                deps = [
                    "github.com/google/go-cmp/cmp/internal/testprotos",
                ],
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/teststructs/foo1",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/teststructs/foo2",
            ),
            package(
                name = "github.com/google/go-cmp/cmp/internal/value",
                test_deps = [
                    "github.com/google/go-cmp/cmp",
                ],
            ),
        ],
    ),
    module(
        name = "github.com/google/osv-scanner",
        version = "v1.4.3",
        packages = [
            package(
                name = "github.com/google/osv-scanner/pkg/models",
                deps = [
                    "github.com/google/go-cmp/cmp",
                    "github.com/package-url/packageurl-go",
                    "golang.org/x/exp/slices",
                    "gopkg.in/yaml.v3",
                ],
            ),
            package(
                name = "github.com/google/osv-scanner/pkg/osv",
                deps = [
                    "github.com/google/osv-scanner/pkg/models",
                    "golang.org/x/sync/semaphore",
                ],
            ),
        ],
        patch_args = ["-p1"],
        patches = [
            "patches/github.com_google_osv-scanner_pkg_osv_osv.go",
        ],
    ),
    module(
        name = "github.com/package-url/packageurl-go",
        version = "v0.1.2",
        packages = [
            package(
                name = "github.com/package-url/packageurl-go",
                no_tests = True,  # The tests require an external file.
            ),
        ],
    ),
    module(
        name = "golang.org/x/crypto",
        version = "v0.15.0",
        packages = [
            package(
                name = "golang.org/x/crypto/ed25519",
            ),
        ],
    ),
    module(
        name = "golang.org/x/exp",
        version = "v0.0.0-20231110203233-9a3e6036ecaa",
        packages = [
            package(
                name = "golang.org/x/exp/constraints",
            ),
            package(
                name = "golang.org/x/exp/slices",
                deps = [
                    "golang.org/x/exp/constraints",
                ],
            ),
        ],
    ),
    module(
        name = "golang.org/x/mod",
        version = "v0.14.0",
        packages = [
            package(
                name = "golang.org/x/mod/internal/lazyregexp",
            ),
            package(
                name = "golang.org/x/mod/module",
                deps = [
                    "golang.org/x/mod/internal/lazyregexp",
                    "golang.org/x/mod/semver",
                    "golang.org/x/xerrors",
                ],
            ),
            package(
                name = "golang.org/x/mod/semver",
            ),
            package(
                name = "golang.org/x/mod/sumdb",
                deps = [
                    "golang.org/x/mod/internal/lazyregexp",
                    "golang.org/x/mod/module",
                    "golang.org/x/mod/sumdb/note",
                    "golang.org/x/mod/sumdb/tlog",
                ],
                test_deps = [
                    "golang.org/x/mod/sumdb/note",
                    "golang.org/x/mod/sumdb/tlog",
                ],
            ),
            package(
                name = "golang.org/x/mod/sumdb/dirhash",
            ),
            package(
                name = "golang.org/x/mod/sumdb/note",
                deps = [
                    "golang.org/x/crypto/ed25519",
                ],
                test_deps = [
                    "golang.org/x/crypto/ed25519",
                ],
            ),
            package(
                name = "golang.org/x/mod/sumdb/tlog",
            ),
            package(
                name = "golang.org/x/mod/zip",
                deps = [
                    "golang.org/x/mod/module",
                ],
                test_size = "medium",
                test_deps = [
                    "golang.org/x/mod/module",
                    "golang.org/x/mod/sumdb/dirhash",
                    "golang.org/x/tools/txtar",
                ],
            ),
        ],
    ),
    module(
        name = "golang.org/x/sync",
        version = "v0.5.0",
        packages = [
            package(
                name = "golang.org/x/sync/errgroup",
            ),
            package(
                name = "golang.org/x/sync/semaphore",
                test_deps = [
                    "golang.org/x/sync/errgroup",
                ],
            ),
        ],
    ),
    module(
        name = "golang.org/x/time",
        version = "v0.4.0",
        packages = [
            package(
                name = "golang.org/x/time/rate",
            ),
        ],
    ),
    module(
        name = "golang.org/x/tools",
        version = "v0.15.0",
        packages = [
            package(
                name = "golang.org/x/tools/txtar",
            ),
        ],
    ),
    module(
        name = "golang.org/x/xerrors",
        version = "v0.0.0-20231012003039-104605ab7028",
        packages = [
            package(
                name = "golang.org/x/xerrors",
                deps = [
                    "golang.org/x/xerrors/internal",
                ],
            ),
            package(
                name = "golang.org/x/xerrors/internal",
            ),
        ],
    ),
    module(
        name = "gopkg.in/yaml.v3",
        version = "v3.0.1",
        packages = [
            package(
                name = "gopkg.in/yaml.v3",
                no_tests = True,  # The tests require more dependencies.
            ),
        ],
    ),
    module(
        name = "rsc.io/diff",
        version = "v0.0.0-20190621135850-fe3479844c3c",
        packages = [
            package(
                name = "rsc.io/diff",
            ),
        ],
    ),
]
