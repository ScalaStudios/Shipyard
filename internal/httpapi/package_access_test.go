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
	jar := "gg/scala/demo/demo-api/1.0.0/demo-api-1.0.0.jar"
	for _, repo := range []string{pub.ID, priv.ID} {
		if _, err := s.packages.PublishOrReplace(ctx, repo, "gg.scala.demo:demo-api", "1.0.0", "demo-api-1.0.0.jar", strings.NewReader("jar"), 3); err != nil {
			t.Fatal(err)
		}
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
		{"authenticated get private jar", http.MethodGet, maven + "private/" + jar, full, http.StatusOK},
		{"authenticated put private", http.MethodPut, maven + "private/" + jar, full, http.StatusCreated},
		{"authenticated put public", http.MethodPut, maven + "public/" + jar, full, http.StatusCreated},
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
}
