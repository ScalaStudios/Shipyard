package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/database"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/storage"
)

func TestPackageRepoAnonymousAccess(t *testing.T) {
	dsn := os.Getenv("SHIPYARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SHIPYARD_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewFilesystemStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(pool, store, Options{})
	h := s.Handler()

	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	id := hex.EncodeToString(suffix)
	user, err := s.identity.CreateUser(ctx, "u"+id, id+"@example.test", "", "correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	org, err := s.orgs.CreateOrganization(ctx, user.ID, "org"+id, "Org", "")
	if err != nil {
		t.Fatal(err)
	}
	project, err := s.orgs.CreateProject(ctx, user.ID, org.ID, "proj", "Proj", "")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := s.packages.CreateRepository(ctx, project.ID, "public", "maven")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.packages.SetRepositoryPublic(ctx, project.ID, pub.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.packages.SetRepositoryAllowedPackages(ctx, project.ID, pub.ID, []string{"gg.scala.demo:demo-api"}); err != nil {
		t.Fatal(err)
	}
	priv, err := s.packages.CreateRepository(ctx, project.ID, "private", "maven")
	if err != nil {
		t.Fatal(err)
	}
	npmPub, err := s.packages.CreateRepository(ctx, project.ID, "npm-public", "npm")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.packages.SetRepositoryPublic(ctx, project.ID, npmPub.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.packages.SetRepositoryAllowedPackages(ctx, project.ID, npmPub.ID, []string{"demo"}); err != nil {
		t.Fatal(err)
	}
	jar := "gg/scala/demo/demo-api/1.0.0/demo-api-1.0.0.jar"
	for _, repo := range []string{pub.ID, priv.ID} {
		if _, err := s.packages.PublishOrReplace(ctx, repo, "gg.scala.demo:demo-api", "1.0.0", "demo-api-1.0.0.jar", strings.NewReader("jar"), 3); err != nil {
			t.Fatal(err)
		}
	}
	secret := "gg/scala/secret/internal/1.0.0/internal-1.0.0.jar"
	if _, err := s.packages.PublishOrReplace(ctx, pub.ID, "gg.scala.secret:internal", "1.0.0", "internal-1.0.0.jar", strings.NewReader("jar"), 3); err != nil {
		t.Fatal(err)
	}
	if _, err := s.packages.Publish(ctx, npmPub.ID, "demo", "1.0.0", "", strings.NewReader("tgz"), 3, nil); err != nil {
		t.Fatal(err)
	}
	full, _, _, err := s.identity.CreateAPIToken(ctx, user.ID, "rw", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	readOnly, _, _, err := s.identity.CreateAPIToken(ctx, user.ID, "ro", nil, []string{identity.ScopeRegistryRead})
	if err != nil {
		t.Fatal(err)
	}

	maven := "/repository/maven/org" + id + "/proj/"
	npm := "/repository/npm/org" + id + "/proj/npm-public/"
	cases := []struct {
		name   string
		method string
		path   string
		token  string
		want   int
	}{
		{"anonymous get public jar", http.MethodGet, maven + "public/" + jar, "", http.StatusOK},
		{"anonymous head public jar", http.MethodHead, maven + "public/" + jar, "", http.StatusOK},
		{"anonymous get public metadata", http.MethodGet, maven + "public/gg/scala/demo/demo-api/maven-metadata.xml", "", http.StatusOK},
		{"anonymous get public metadata sha1", http.MethodGet, maven + "public/gg/scala/demo/demo-api/maven-metadata.xml.sha1", "", http.StatusOK},
		{"anonymous get public npm packument", http.MethodGet, npm + "demo", "", http.StatusOK},
		{"anonymous get public npm tarball", http.MethodGet, npm + "demo/-/demo-1.0.0.tgz", "", http.StatusOK},
		{"anonymous get private jar", http.MethodGet, maven + "private/" + jar, "", http.StatusUnauthorized},
		{"anonymous get private metadata", http.MethodGet, maven + "private/gg/scala/demo/demo-api/maven-metadata.xml", "", http.StatusUnauthorized},
		{"anonymous get missing repo", http.MethodGet, maven + "nope/" + jar, "", http.StatusUnauthorized},
		{"anonymous put public", http.MethodPut, maven + "public/" + jar, "", http.StatusUnauthorized},
		{"anonymous put private", http.MethodPut, maven + "private/" + jar, "", http.StatusUnauthorized},
		{"anonymous npm publish public", http.MethodPut, npm + "demo", "", http.StatusUnauthorized},
		{"anonymous get public missing metadata", http.MethodGet, maven + "public/gg/scala/demo/missing/maven-metadata.xml", "", http.StatusNotFound},
		{"anonymous get public missing metadata sha1", http.MethodGet, maven + "public/gg/scala/demo/missing/maven-metadata.xml.sha1", "", http.StatusNotFound},
		{"anonymous get public snapshot version metadata", http.MethodGet, maven + "public/gg/scala/demo/demo-api/1.0.0-SNAPSHOT/maven-metadata.xml", "", http.StatusNotFound},
		{"anonymous get private missing metadata", http.MethodGet, maven + "private/gg/scala/demo/missing/maven-metadata.xml", "", http.StatusUnauthorized},
		{"authenticated get private metadata", http.MethodGet, maven + "private/gg/scala/demo/demo-api/maven-metadata.xml", full, http.StatusOK},
		{"authenticated get private missing metadata", http.MethodGet, maven + "private/gg/scala/demo/missing/maven-metadata.xml", full, http.StatusNotFound},
		{"authenticated get private jar", http.MethodGet, maven + "private/" + jar, full, http.StatusOK},
		{"authenticated put private", http.MethodPut, maven + "private/" + jar, full, http.StatusCreated},
		{"authenticated put public", http.MethodPut, maven + "public/" + jar, full, http.StatusCreated},
		{"anonymous get unlisted package in public repo", http.MethodGet, maven + "public/" + secret, "", http.StatusNotFound},
		{"anonymous get unlisted metadata in public repo", http.MethodGet, maven + "public/gg/scala/secret/internal/maven-metadata.xml", "", http.StatusNotFound},
		{"anonymous get unlisted npm package in public repo", http.MethodGet, npm + "other", "", http.StatusNotFound},
		{"authenticated get unlisted package in public repo", http.MethodGet, maven + "public/" + secret, full, http.StatusOK},
		{"authenticated put unlisted to public", http.MethodPut, maven + "public/gg/scala/other/lib/1.0.0/lib-1.0.0.jar", full, http.StatusForbidden},
		{"authenticated put unlisted metadata to public", http.MethodPut, maven + "public/gg/scala/other/lib/maven-metadata.xml", full, http.StatusForbidden},
		{"authenticated put nested path under allowed package", http.MethodPut, maven + "public/gg/scala/demo/demo-api/nested/1.0.0/nested-1.0.0.jar", full, http.StatusForbidden},
		{"authenticated put unlisted to private", http.MethodPut, maven + "private/gg/scala/other/lib/1.0.0/lib-1.0.0.jar", full, http.StatusCreated},
		{"authenticated npm publish allowed to public", http.MethodPut, npm + "demo?version=2.0.0", full, http.StatusCreated},
		{"authenticated npm publish unlisted to public", http.MethodPut, npm + "other?version=1.0.0", full, http.StatusForbidden},
		{"read-only token put private", http.MethodPut, maven + "private/" + jar, readOnly, http.StatusForbidden},
		{"read-only token put public", http.MethodPut, maven + "public/" + jar, readOnly, http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader("jar"))
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("got %d want %d: %s", rec.Code, c.want, rec.Body.String())
			}
			if c.want == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
				t.Fatalf("missing WWW-Authenticate")
			}
		})
	}
	if _, err := pool.Exec(ctx, `UPDATE package_versions SET protected = true WHERE repository_id = $1 AND filename = 'demo-api-1.0.0.jar'`, priv.ID); err != nil {
		t.Fatal(err)
	}
	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, maven+"private/"+jar, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+full)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	t.Run("protected re-upload with identical bytes succeeds", func(t *testing.T) {
		if rec := put("jar"); rec.Code != http.StatusCreated {
			t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("protected re-upload with different bytes is rejected", func(t *testing.T) {
		rec := put("tampered")
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "is protected") {
			t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
		}
		req := httptest.NewRequest(http.MethodGet, maven+"private/"+jar, nil)
		req.Header.Set("Authorization", "Bearer "+full)
		got := httptest.NewRecorder()
		h.ServeHTTP(got, req)
		if got.Body.String() != "jar" {
			t.Fatalf("stored bytes changed to %q", got.Body.String())
		}
		var events int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action = 'package_version.protected_overwrite_rejected' AND actor_user_id = $1`, user.ID).Scan(&events); err != nil {
			t.Fatal(err)
		}
		if events != 1 {
			t.Fatalf("expected 1 audit event, got %d", events)
		}
	})

	var rejected int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action = 'package_repository.upload_rejected' AND resource_id IN ($1, $2)`, pub.ID, npmPub.ID).Scan(&rejected); err != nil {
		t.Fatal(err)
	}
	if rejected != 4 {
		t.Fatalf("expected 4 rejected uploads in audit log, got %d", rejected)
	}
}
