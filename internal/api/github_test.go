package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/shurcooL/githubv4"
)

func TestListLegacyMigrationsPaginatesResourcesPerMigration(t *testing.T) {
	resource := func(name string) string {
		return fmt.Sprintf(`{"targetUrl":"https://github.com/o/%s","modelName":"repository"}`, name)
	}
	graphql := func(nodes, cursor string, hasNext bool) string {
		return fmt.Sprintf(`{"data":{"organization":{"migration":{"migratableResources":{"nodes":[%s],"pageInfo":{"hasNextPage":%t,"endCursor":"%s"}}}}}}`, nodes, hasNext, cursor)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/migrations") {
			fmt.Fprint(w, `[{"guid":"g1","state":"exported"},{"guid":"g2","state":"exported"}]`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.Contains(string(body), `"guid":"g1"`) && strings.Contains(string(body), `"after":null`):
			fmt.Fprint(w, graphql(resource("a")+","+resource("b"), "C1", true))
		case strings.Contains(string(body), `"guid":"g1"`) && strings.Contains(string(body), `"after":"C1"`):
			fmt.Fprint(w, graphql(resource("c"), "C2", false))
		case strings.Contains(string(body), `"guid":"g2"`) && strings.Contains(string(body), `"after":null`):
			fmt.Fprint(w, graphql(resource("d"), "", false))
		default:
			t.Errorf("unexpected request: %s", body)
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	baseURL, _ := url.Parse(srv.URL + "/")
	rest := github.NewClient(nil)
	rest.BaseURL = baseURL
	c := &githubClient{
		restClient:    rest,
		graphqlClient: githubv4.NewEnterpriseClient(srv.URL, http.DefaultClient),
		pageSize:      2,
	}

	migrations, err := c.listLegacyMigrations(context.Background(), "o", nil)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, m := range migrations {
		got = append(got, m.ID+":"+m.RepositoryName)
	}
	want := "g1:https://github.com/o/a g1:https://github.com/o/b g1:https://github.com/o/c g2:https://github.com/o/d"
	if strings.Join(got, " ") != want {
		t.Errorf("got %v, want %s", got, want)
	}
}

// GitHub's repositoryMigrations "after" cursor is inclusive: the page after
// cursor X starts with X itself.
func TestListGEIMigrationsHandlesInclusiveCursor(t *testing.T) {
	ids := []string{"m1", "m2", "m3", "m4"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				First int
				After *string
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}

		start := 0
		if req.Variables.After != nil {
			for i, id := range ids {
				if id == *req.Variables.After {
					start = i
				}
			}
		}
		end := start + req.Variables.First
		if end > len(ids) {
			end = len(ids)
		}

		var edges []string
		for _, id := range ids[start:end] {
			edges = append(edges, fmt.Sprintf(`{"node":{"id":"%s","repositoryName":"%s","state":"SUCCEEDED","createdAt":"2024-01-01T00:00:00Z"}}`, id, id))
		}
		fmt.Fprintf(w, `{"data":{"organization":{"repositoryMigrations":{"pageInfo":{"endCursor":"%s","hasNextPage":%t},"edges":[%s]}}}}`,
			ids[end-1], end < len(ids), strings.Join(edges, ","))
	}))
	defer srv.Close()

	for _, pageSize := range []int{1, 2, 3, 100} {
		c := &githubClient{
			graphqlClient: githubv4.NewEnterpriseClient(srv.URL, http.DefaultClient),
			pageSize:      pageSize,
		}
		migrations, err := c.listGEIMigrations(context.Background(), "o", nil)
		if err != nil {
			t.Fatal(err)
		}

		var got []string
		for _, m := range migrations {
			got = append(got, m.ID)
		}
		if strings.Join(got, ",") != strings.Join(ids, ",") {
			t.Errorf("page size %d: got %v, want %v", pageSize, got, ids)
		}
	}
}

func TestListGEIMigrationsFiltersOlderThanCutoff(t *testing.T) {
	ids := []string{"m1", "m2", "m3", "m4"}
	times := map[string]string{
		"m1": "2024-01-01T00:00:00Z",
		"m2": "2024-02-01T00:00:00Z",
		"m3": "2024-03-01T00:00:00Z",
		"m4": "2024-04-01T00:00:00Z",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				First int
				After *string
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}

		start := 0
		if req.Variables.After != nil {
			for i, id := range ids {
				if id == *req.Variables.After {
					start = i + 1
				}
			}
		}
		end := start + req.Variables.First
		if end > len(ids) {
			end = len(ids)
		}

		var edges []string
		for _, id := range ids[start:end] {
			edges = append(edges, fmt.Sprintf(`{"node":{"id":"%s","repositoryName":"%s","state":"SUCCEEDED","createdAt":"%s"}}`, id, id, times[id]))
		}
		fmt.Fprintf(w, `{"data":{"organization":{"repositoryMigrations":{"pageInfo":{"endCursor":"%s","hasNextPage":%t},"edges":[%s]}}}}`,
			ids[end-1], end < len(ids), strings.Join(edges, ","))
	}))
	defer srv.Close()

	cutoff, _ := time.Parse(time.RFC3339, "2024-02-15T00:00:00Z")
	c := &githubClient{
		graphqlClient: githubv4.NewEnterpriseClient(srv.URL, http.DefaultClient),
		pageSize:      2,
		since:         func() time.Time { return cutoff },
	}

	migrations, err := c.listGEIMigrations(context.Background(), "o", nil)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, m := range migrations {
		got = append(got, m.ID)
	}
	if strings.Join(got, ",") != "m3,m4" {
		t.Errorf("got %v, want [m3 m4]", got)
	}
}
