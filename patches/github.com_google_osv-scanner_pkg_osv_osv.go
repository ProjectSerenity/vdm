diff --git a/pkg/osv/osv.go b/pkg/osv/osv.go
index 6fe7443..5bc8234 100644
--- a/pkg/osv/osv.go
+++ b/pkg/osv/osv.go
@@ -10,7 +10,6 @@ import (
 	"net/http"
 	"time"

-	"github.com/google/osv-scanner/pkg/lockfile"
 	"github.com/google/osv-scanner/pkg/models"
 	"golang.org/x/sync/semaphore"
 )
@@ -118,26 +117,6 @@ func MakePURLRequest(purl string) *Query {
 	}
 }

-func MakePkgRequest(pkgDetails lockfile.PackageDetails) *Query {
-	// API has trouble parsing requests with both commit and Package details filled in
-	if pkgDetails.Ecosystem == "" && pkgDetails.Commit != "" {
-		return &Query{
-			Metadata: models.Metadata{
-				RepoURL: pkgDetails.Name,
-			},
-			Commit: pkgDetails.Commit,
-		}
-	} else {
-		return &Query{
-			Version: pkgDetails.Version,
-			Package: Package{
-				Name:      pkgDetails.Name,
-				Ecosystem: string(pkgDetails.Ecosystem),
-			},
-		}
-	}
-}
-
 // From: https://stackoverflow.com/a/72408490
 func chunkBy[T any](items []T, chunkSize int) [][]T {
 	chunks := make([][]T, 0, (len(items)/chunkSize)+1)