diff --git a/build/build_defs.bzl b/build/build_defs.bzl
index 0446260..db2febb 100644
--- a/build/build_defs.bzl
+++ b/build/build_defs.bzl
@@ -16,7 +16,7 @@ distributed under the License is distributed on an "AS IS" BASIS,
 """
 
 load(
-    "@io_bazel_rules_go//go:def.bzl",
+    "@rules_go//go:def.bzl",
     "GoSource",
 )
 
