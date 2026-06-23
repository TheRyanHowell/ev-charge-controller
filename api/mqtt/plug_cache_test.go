package mqtt

import (
	"context"
	"testing"

	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPlugRepoCache struct {
	plugs map[string]*models.Plug
	err   error
}

func (m *mockPlugRepoCache) FindByNamespaceAndSlug(_ context.Context, namespace, slug string) (*models.Plug, error) {
	for _, p := range m.plugs {
		if p.Namespace == namespace && p.MqttTopic == slug {
			return p, nil
		}
	}
	return nil, m.err
}

func (m *mockPlugRepoCache) FindByID(_ context.Context, id string) (*models.Plug, error) {
	for _, p := range m.plugs {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, m.err
}

func (m *mockPlugRepoCache) List(_ context.Context, _ string) ([]models.Plug, error) {
	var result []models.Plug
	for _, p := range m.plugs {
		result = append(result, *p)
	}
	return result, m.err
}

func (m *mockPlugRepoCache) Create(_ context.Context, plug *models.Plug) error {
	m.plugs[plug.ID] = plug
	return m.err
}

func (m *mockPlugRepoCache) Update(_ context.Context, plug *models.Plug) error {
	m.plugs[plug.ID] = plug
	return m.err
}

func (m *mockPlugRepoCache) Delete(_ context.Context, id, _ string) error {
	delete(m.plugs, id)
	return m.err
}

func (m *mockPlugRepoCache) SetOnline(_ context.Context, plugID string, online bool) error {
	return m.err
}

func (m *mockPlugRepoCache) UpdateLastOfflineNotifiedAt(_ context.Context, plugID string) error {
	return m.err
}

func (m *mockPlugRepoCache) SetInitialized(_ context.Context, plugID string) error {
	return m.err
}

func (m *mockPlugRepoCache) SetPowerState(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockPlugRepoCache) ListNamespacesByUserID(_ context.Context, _ string) ([]string, error) {
	return nil, m.err
}

func TestNewPlugCache(t *testing.T) {
	repo := &mockPlugRepoCache{plugs: make(map[string]*models.Plug)}
	cache := NewPlugCache(repo)
	require.NotNil(t, cache)
	assert.NotNil(t, cache.m)
	assert.Equal(t, repo, cache.repo)
}

func TestPlugCache_Lookup_CacheHit(t *testing.T) {
	repo := &mockPlugRepoCache{plugs: make(map[string]*models.Plug)}
	cache := NewPlugCache(repo)
	cache.m[NamespaceSlug{Namespace: "ns1", Slug: "plug1"}] = "plug-id-1"

	id, ok := cache.Lookup("ns1", "plug1")
	assert.True(t, ok)
	assert.Equal(t, "plug-id-1", id)
}

func TestPlugCache_Lookup_CacheMiss_LoadFromDB(t *testing.T) {
	repo := &mockPlugRepoCache{
		plugs: map[string]*models.Plug{
			"plug-id-1": {ID: "plug-id-1", Namespace: "ns1", MqttTopic: "plug1"},
		},
	}
	cache := NewPlugCache(repo)

	id, ok := cache.Lookup("ns1", "plug1")
	assert.True(t, ok)
	assert.Equal(t, "plug-id-1", id)

	// Second lookup should hit cache
	id2, ok2 := cache.Lookup("ns1", "plug1")
	assert.True(t, ok2)
	assert.Equal(t, "plug-id-1", id2)
}

func TestPlugCache_Lookup_CacheMiss_NotInDB(t *testing.T) {
	repo := &mockPlugRepoCache{plugs: make(map[string]*models.Plug)}
	cache := NewPlugCache(repo)

	id, ok := cache.Lookup("ns1", "plug1")
	assert.False(t, ok)
	assert.Empty(t, id)
}

func TestPlugCache_Lookup_CacheMiss_DBError(t *testing.T) {
	repo := &mockPlugRepoCache{
		plugs: make(map[string]*models.Plug),
		err:   assert.AnError,
	}
	cache := NewPlugCache(repo)

	id, ok := cache.Lookup("ns1", "plug1")
	assert.False(t, ok)
	assert.Empty(t, id)
}

func TestPlugCache_Lookup_NoRepo(t *testing.T) {
	cache := NewStaticPlugCache(nil)

	id, ok := cache.Lookup("ns1", "plug1")
	assert.False(t, ok)
	assert.Empty(t, id)
}

func TestNewStaticPlugCache(t *testing.T) {
	entries := map[NamespaceSlug]string{
		{Namespace: "ns1", Slug: "plug1"}: "plug-id-1",
		{Namespace: "ns2", Slug: "plug2"}: "plug-id-2",
	}
	cache := NewStaticPlugCache(entries)
	require.NotNil(t, cache)
	assert.Len(t, cache.m, 2)
	assert.Equal(t, "plug-id-1", cache.m[NamespaceSlug{Namespace: "ns1", Slug: "plug1"}])
}

func TestPlugCache_Invalidate(t *testing.T) {
	cache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns1", Slug: "plug1"}: "plug-id-1",
	})

	_, ok := cache.Lookup("ns1", "plug1")
	assert.True(t, ok)

	cache.Invalidate("ns1", "plug1")
	_, ok = cache.Lookup("ns1", "plug1")
	assert.False(t, ok)
}

func TestPlugCache_Invalidate_NonExistent(t *testing.T) {
	cache := NewStaticPlugCache(nil)
	cache.Invalidate("ns1", "plug1")
}
