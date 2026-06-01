package emulator_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/emulator"
)

// TestClient_ListGroups проверяет разбор списка групп (обёртка data+meta).
func TestClient_ListGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/groups", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"g1","name":"ПИ-01","external_id":"EXT-GRP-001","faculty":"Факультет информатики","course":1,"flow_name":"Поток 1"},
				{"id":"g2","name":"ПИ-02","external_id":"EXT-GRP-002","faculty":"Факультет информатики","course":2,"flow_name":"Поток 2"}
			],
			"meta":{"page":1,"limit":100,"total":2,"total_pages":1}
		}`))
	}))
	defer srv.Close()

	c := emulator.New(srv.URL, "test-key")
	items, meta, err := c.ListGroups(context.Background(), 1, 100)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "g1", items[0].ID)
	require.Equal(t, "ПИ-01", items[0].Name)
	require.Equal(t, 1, items[0].Course)
	require.Equal(t, 1, meta.TotalPages)
}

// TestClient_GetGroup проверяет ключевой нюанс: одиночный endpoint
// /groups/{id} отдаёт объект БЕЗ обёртки {"data":...}, поэтому декодим
// напрямую в GroupDTO.
func TestClient_GetGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasPrefix(r.URL.Path, "/api/v1/groups/"))
		w.Header().Set("Content-Type", "application/json")
		// Намеренно без обёртки data — как реальный эмулятор.
		_, _ = w.Write([]byte(`{"id":"g2","name":"ПИ-02","external_id":"EXT-GRP-002","faculty":"Факультет информатики","course":2,"flow_name":"Поток 2"}`))
	}))
	defer srv.Close()

	c := emulator.New(srv.URL, "test-key")
	g, err := c.GetGroup(context.Background(), "g2")
	require.NoError(t, err)
	require.Equal(t, "g2", g.ID)
	require.Equal(t, "ПИ-02", g.Name)
}

// TestClient_ListAccounts_ParsesGroupID убеждается, что добавленное поле
// group_id десериализуется из аккаунта (источник группы студента).
func TestClient_ListAccounts_ParsesGroupID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"acc1","email":"s@example.com","first_name":"Имя","last_name":"Фам","role":"student","status":"active","password_hash":"$2b$12$x","linked_entity_id":"stu-1","group_id":"g2"}
			],
			"meta":{"page":1,"limit":100,"total":1,"total_pages":1}
		}`))
	}))
	defer srv.Close()

	c := emulator.New(srv.URL, "test-key")
	items, _, err := c.ListAccounts(context.Background(), "student", 1, 100)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "g2", items[0].GroupID, "group_id должен десериализоваться из аккаунта")
}
